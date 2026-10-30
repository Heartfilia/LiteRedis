package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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
	if err := os.MkdirAll(appDir, 0700); err != nil {
		// fallback to temp
		appDir = os.TempDir()
	}
	configPath = filepath.Join(appDir, "connections.json")
}

// ConfigPath 配置文件路径（诊断页展示、排查问题时用）
func ConfigPath() string {
	return configPath
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
	if err := secureConfigPermissions(configPath); err != nil {
		return nil, fmt.Errorf("secure config permissions: %w", err)
	}
	var store ConfigStore
	if err := json.Unmarshal(data, &store); err != nil {
		backupPath, backupErr := backupCorruptStore(data)
		if backupErr != nil {
			return nil, fmt.Errorf("invalid config JSON at %s: %w (backup failed: %v)", configPath, err, backupErr)
		}
		return nil, fmt.Errorf("invalid config JSON at %s; original preserved and backup saved to %s: %w", configPath, backupPath, err)
	}
	// 迁移：若旧数据没有 Settings 字段，补默认值
	if store.Settings.KeyScanCount == 0 || store.Settings.SearchHistoryLimit == 0 {
		store.Settings = DefaultSettings()
	}
	// 旧配置缺少本地 API 字段（port 未配置过为 0）：按默认关闭处理，需要用户显式开启
	if store.Settings.LocalAPIPort == 0 {
		store.Settings.LocalAPIEnabled = false
		store.Settings.LocalAPIPort = DefaultSettings().LocalAPIPort
	}
	// 一次性迁移（保存设置时打标，之后不再执行）：
	// 1. 历史默认的每页 20 条提到 200
	// 2. 本地数据提取 API 改为默认关闭，需要进开发者模式手动开启
	if !store.Settings.ScanDefaultMigrated {
		if store.Settings.KeyScanCount == 20 {
			store.Settings.KeyScanCount = 200
		}
		store.Settings.LocalAPIEnabled = false
		store.Settings.ScanDefaultMigrated = true
	}
	ensureUniqueConnectionNames(&store)
	ensureConnectionOrder(&store)
	return &store, nil
}

// ensureUniqueConnectionNames 一次性整理历史数据：同一分组下的重名连接自动加序号后缀，
// 保证「同组名称唯一」的规则对老配置同样成立（结果会在下一次保存时落盘）
func ensureUniqueConnectionNames(store *ConfigStore) {
	if len(store.Connections) < 2 {
		return
	}
	seen := make(map[string]struct{}, len(store.Connections))
	for i := range store.Connections {
		conn := &store.Connections[i]
		name := strings.TrimSpace(conn.Name)
		group := strings.TrimSpace(conn.Group)
		conn.Group = group
		if name == "" {
			// 空名字属于脏数据，留给用户自己改，这里不硬塞名字
			continue
		}
		key := strings.ToLower(group) + "\x00" + strings.ToLower(name)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			conn.Name = name
			continue
		}
		for suffix := 2; ; suffix++ {
			candidate := fmt.Sprintf("%s (%d)", name, suffix)
			candidateKey := strings.ToLower(group) + "\x00" + strings.ToLower(candidate)
			if _, taken := seen[candidateKey]; taken {
				continue
			}
			seen[candidateKey] = struct{}{}
			conn.Name = candidate
			break
		}
	}
}

func backupCorruptStore(data []byte) (string, error) {
	modifiedAt := time.Now()
	if info, err := os.Stat(configPath); err == nil {
		modifiedAt = info.ModTime()
	}
	digest := sha256.Sum256(data)
	stamp := modifiedAt.UTC().Format("20060102T150405.000000000Z")
	backupPath := fmt.Sprintf("%s.corrupt-%s-%x.bak", configPath, stamp, digest[:6])
	if _, err := os.Stat(backupPath); err == nil {
		return backupPath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := AtomicWriteFile(backupPath, data, 0600); err != nil {
		return "", err
	}
	return backupPath, nil
}

// saveStore 写入磁盘（不加锁，调用方负责）
func saveStore(store *ConfigStore) error {
	ensureConnectionOrder(store)
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(configPath, data, 0600)
}

func secureConfigPermissions(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, 0600)
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

	// 同一分组下名称唯一：忽略首尾空格与大小写，避免出现两个看起来一样的连接
	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Group = strings.TrimSpace(cfg.Group)
	if cfg.Name == "" {
		return cfg, fmt.Errorf("连接名称不能为空")
	}
	for _, existing := range store.Connections {
		if existing.ID == cfg.ID {
			continue
		}
		if strings.TrimSpace(existing.Group) != cfg.Group {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(existing.Name), cfg.Name) {
			if cfg.Group == "" {
				return cfg, fmt.Errorf("默认分组下已存在同名连接「%s」", cfg.Name)
			}
			return cfg, fmt.Errorf("分组「%s」下已存在同名连接「%s」", cfg.Group, cfg.Name)
		}
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

	// 拖拽到别的分组后同样要保证同组名称唯一，否则直接拒绝这次排序
	seen := make(map[string]struct{}, len(store.Connections))
	for _, conn := range store.Connections {
		key := strings.ToLower(strings.TrimSpace(conn.Group)) + "\x00" + strings.ToLower(strings.TrimSpace(conn.Name))
		if _, exists := seen[key]; exists {
			group := strings.TrimSpace(conn.Group)
			if group == "" {
				group = "默认分组"
			}
			return fmt.Errorf("分组「%s」下已存在同名连接「%s」，请先改名再移动", group, strings.TrimSpace(conn.Name))
		}
		seen[key] = struct{}{}
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
	settings := store.Settings
	// 端口按构建类型归一化：开发版用 64202、正式版用 64201，两个版本共享同一份配置也不会撞端口
	settings.LocalAPIPort = EffectiveLocalAPIPort(settings.LocalAPIPort)
	return settings, nil
}

// SaveSettings 保存全局设置
func SaveSettings(s AppSettings) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return err
	}
	defaults := DefaultSettings()
	// 用户显式保存过设置，标记迁移完成，后续不再把 20 改成 200
	s.ScanDefaultMigrated = true
	// 边界保护：非法值回退到与新配置一致的默认值。
	if s.KeyScanCount <= 0 {
		s.KeyScanCount = defaults.KeyScanCount
	}
	if s.KeyScanCount > 10000 {
		s.KeyScanCount = 10000
	}
	if s.HashLoadCount <= 0 {
		s.HashLoadCount = defaults.HashLoadCount
	}
	if s.HashLoadCount > 10000 {
		s.HashLoadCount = 10000
	}
	if s.ListLoadCount <= 0 {
		s.ListLoadCount = defaults.ListLoadCount
	}
	if s.ListLoadCount > 10000 {
		s.ListLoadCount = 10000
	}
	if s.SetLoadCount <= 0 {
		s.SetLoadCount = defaults.SetLoadCount
	}
	if s.SetLoadCount > 10000 {
		s.SetLoadCount = 10000
	}
	if s.ZSetLoadCount <= 0 {
		s.ZSetLoadCount = defaults.ZSetLoadCount
	}
	if s.ZSetLoadCount > 10000 {
		s.ZSetLoadCount = 10000
	}
	if s.StreamLoadCount <= 0 {
		s.StreamLoadCount = defaults.StreamLoadCount
	}
	if s.StreamLoadCount > 10000 {
		s.StreamLoadCount = 10000
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
	// 本地 API：端口范围 1024-65535，非法回退默认；enabled 是独立开关不在此强制
	if s.LocalAPIPort < 1024 || s.LocalAPIPort > 65535 {
		s.LocalAPIPort = defaults.LocalAPIPort
	}
	// 保存设置时保留「已看过更新说明的版本」与「已看过引导」，避免设置一保存又弹一次
	if s.LastSeenVersion == "" {
		s.LastSeenVersion = store.Settings.LastSeenVersion
	}
	if !s.OnboardingDone {
		s.OnboardingDone = store.Settings.OnboardingDone
	}
	// 命令历史同理：前端保存设置不会带这个字段，清空要显式调用 ClearCommandHistory
	if s.CommandHistory == nil {
		s.CommandHistory = store.Settings.CommandHistory
	}
	store.Settings = s
	return saveStore(store)
}

// MarkOnboardingDone 记录首次使用向导已看过（之后不再弹）
func MarkOnboardingDone() error {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return err
	}
	if store.Settings.OnboardingDone {
		return nil
	}
	store.Settings.OnboardingDone = true
	return saveStore(store)
}

// MarkVersionSeen 记录用户已经看过某个版本的更新说明（每个版本只提示一次）
func MarkVersionSeen(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil
	}
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return err
	}
	if store.Settings.LastSeenVersion == version {
		return nil
	}
	store.Settings.LastSeenVersion = version
	return saveStore(store)
}
