package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	storeMu    sync.RWMutex
	configPath string
)

func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	appDir := filepath.Join(dir, "LiteRedis")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		// fallback to temp
		appDir = os.TempDir()
	}
	configPath = filepath.Join(appDir, "connections.json")
}

// loadStore 从磁盘读取配置（不加锁，调用方负责）
func loadStore() (*ConfigStore, error) {
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return &ConfigStore{Version: 1, Connections: []ConnectionConfig{}, Settings: DefaultSettings()}, nil
	}
	if err != nil {
		return nil, err
	}
	var store ConfigStore
	if err := json.Unmarshal(data, &store); err != nil {
		return &ConfigStore{Version: 1, Connections: []ConnectionConfig{}, Settings: DefaultSettings()}, nil
	}
	// 迁移：若旧数据没有 Settings 字段，补默认值
	if store.Settings.KeyScanCount == 0 || store.Settings.SearchHistoryLimit == 0 {
		store.Settings = DefaultSettings()
	}
	ensureConnectionOrder(&store)
	return &store, nil
}

// saveStore 写入磁盘（不加锁，调用方负责）
func saveStore(store *ConfigStore) error {
	ensureConnectionOrder(store)
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(configPath, data, 0644)
}

func ensureConnectionOrder(store *ConfigStore) {
	if store == nil {
		return
	}
	sort.SliceStable(store.Connections, func(i, j int) bool {
		left := store.Connections[i]
		right := store.Connections[j]
		if left.SortOrder != right.SortOrder {
			return left.SortOrder < right.SortOrder
		}
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.ID < right.ID
	})
	for i := range store.Connections {
		store.Connections[i].SortOrder = i
	}
}

// ListConnections 返回所有连接配置
func ListConnections() ([]ConnectionConfig, error) {
	storeMu.RLock()
	defer storeMu.RUnlock()
	store, err := loadStore()
	if err != nil {
		return nil, err
	}
	ensureConnectionOrder(store)
	return store.Connections, nil
}

// SaveConnection 新建（ID为空）或更新连接配置
func SaveConnection(cfg ConnectionConfig) (ConnectionConfig, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return cfg, err
	}

	now := time.Now()
	if cfg.ID == "" {
		// 新建
		cfg.ID = uuid.New().String()
		cfg.CreatedAt = now
		cfg.UpdatedAt = now
		if cfg.Port == 0 {
			cfg.Port = 6379
		}
		if cfg.SSH != nil && cfg.SSH.Port == 0 {
			cfg.SSH.Port = 22
		}
		cfg.SortOrder = len(store.Connections)
		store.Connections = append(store.Connections, cfg)
	} else {
		// 更新
		found := false
		for i, c := range store.Connections {
			if c.ID == cfg.ID {
				cfg.CreatedAt = c.CreatedAt
				cfg.UpdatedAt = now
				cfg.SortOrder = c.SortOrder
				store.Connections[i] = cfg
				found = true
				break
			}
		}
		if !found {
			cfg.CreatedAt = now
			cfg.UpdatedAt = now
			cfg.SortOrder = len(store.Connections)
			store.Connections = append(store.Connections, cfg)
		}
	}

	return cfg, saveStore(store)
}

func ReorderConnections(items []ConnectionOrderItem) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}

	orderByID := make(map[string]ConnectionOrderItem, len(items))
	for _, item := range items {
		orderByID[item.ID] = item
	}

	for i, conn := range store.Connections {
		if item, ok := orderByID[conn.ID]; ok {
			store.Connections[i].Group = item.Group
			store.Connections[i].SortOrder = item.SortOrder
			store.Connections[i].UpdatedAt = time.Now()
		}
	}

	return saveStore(store)
}

// DeleteConnection 删除连接配置
func DeleteConnection(id string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}

	newConns := store.Connections[:0]
	for _, c := range store.Connections {
		if c.ID != id {
			newConns = append(newConns, c)
		}
	}
	store.Connections = newConns
	return saveStore(store)
}

// GetConnection 通过 ID 获取连接配置
func GetConnection(id string) (*ConnectionConfig, error) {
	storeMu.RLock()
	defer storeMu.RUnlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}
	for _, c := range store.Connections {
		if c.ID == id {
			cc := c
			return &cc, nil
		}
	}
	return nil, nil
}

// GetSettings 获取全局设置
func GetSettings() (AppSettings, error) {
	storeMu.RLock()
	defer storeMu.RUnlock()
	store, err := loadStore()
	if err != nil {
		return DefaultSettings(), err
	}
	return store.Settings, nil
}

// SaveSettings 保存全局设置
func SaveSettings(s AppSettings) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return err
	}
	// 边界保护：不允许为 0
	if s.KeyScanCount <= 0 {
		s.KeyScanCount = 100
	}
	if s.HashLoadCount <= 0 {
		s.HashLoadCount = 200
	}
	if s.ListLoadCount <= 0 {
		s.ListLoadCount = 100
	}
	if s.SetLoadCount <= 0 {
		s.SetLoadCount = 100
	}
	if s.ZSetLoadCount <= 0 {
		s.ZSetLoadCount = 100
	}
	if s.StreamLoadCount <= 0 {
		s.StreamLoadCount = 100
	}
	if s.SearchHistoryLimit <= 0 {
		s.SearchHistoryLimit = 10
	}
	if s.SearchHistoryLimit > 100 {
		s.SearchHistoryLimit = 100
	}
	if s.KeyDisplayMode == "" {
		s.KeyDisplayMode = "tree"
	}
	if s.FontSizeLevel != "small" && s.FontSizeLevel != "medium" && s.FontSizeLevel != "large" {
		s.FontSizeLevel = "small"
	}
	if s.WatermarkSize < 10 {
		s.WatermarkSize = 10
	}
	if s.WatermarkSize > 48 {
		s.WatermarkSize = 48
	}
	if s.WatermarkAngle < -90 {
		s.WatermarkAngle = -90
	}
	if s.WatermarkAngle > 90 {
		s.WatermarkAngle = 90
	}
	if s.WatermarkOpacity < 1 {
		s.WatermarkOpacity = 1
	}
	if s.WatermarkOpacity > 100 {
		s.WatermarkOpacity = 100
	}
	if s.WatermarkDensity < 1 {
		s.WatermarkDensity = 1
	}
	if s.WatermarkDensity > 5 {
		s.WatermarkDensity = 5
	}
	if s.Language == "" {
		s.Language = "zh"
	}
	store.Settings = s
	return saveStore(store)
}
