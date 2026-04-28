package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	return &store, nil
}

// saveStore 写入磁盘（不加锁，调用方负责）
func saveStore(store *ConfigStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// ListConnections 返回所有连接配置
func ListConnections() ([]ConnectionConfig, error) {
	storeMu.RLock()
	defer storeMu.RUnlock()
	store, err := loadStore()
	if err != nil {
		return nil, err
	}
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
		store.Connections = append(store.Connections, cfg)
	} else {
		// 更新
		found := false
		for i, c := range store.Connections {
			if c.ID == cfg.ID {
				cfg.CreatedAt = c.CreatedAt
				cfg.UpdatedAt = now
				store.Connections[i] = cfg
				found = true
				break
			}
		}
		if !found {
			cfg.CreatedAt = now
			cfg.UpdatedAt = now
			store.Connections = append(store.Connections, cfg)
		}
	}

	return cfg, saveStore(store)
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
	if s.Language == "" {
		s.Language = "zh"
	}
	store.Settings = s
	return saveStore(store)
}
