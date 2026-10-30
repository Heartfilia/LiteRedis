package config

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ConfigExport 连接配置导出文件结构（开源项目，密码按现有约定以明文写出，导出前会二次确认）
type ConfigExport struct {
	Version     int                `json:"version"`
	App         string             `json:"app"`
	ExportedAt  string             `json:"exported_at"`
	Connections []ConnectionConfig `json:"connections"`
}

// ImportSummary 导入结果，用于界面提示
type ImportSummary struct {
	Added      int      `json:"added"`
	Updated    int      `json:"updated"`
	Skipped    int      `json:"skipped"`
	Renamed    int      `json:"renamed"`
	UpdatedIDs []string `json:"updated_ids,omitempty"`
	Messages   []string `json:"messages,omitempty"`
}

// ExportConnections 取出全部连接配置（调用方负责写文件与二次确认）
func ExportConnections() (ConfigExport, error) {
	storeMu.RLock()
	defer storeMu.RUnlock()

	store, err := loadStore()
	if err != nil {
		return ConfigExport{}, err
	}
	conns := make([]ConnectionConfig, len(store.Connections))
	copy(conns, store.Connections)
	sort.SliceStable(conns, func(i, j int) bool { return conns[i].SortOrder < conns[j].SortOrder })

	return ConfigExport{
		Version:     1,
		App:         "LiteRedis",
		ExportedAt:  time.Now().Format(time.RFC3339),
		Connections: conns,
	}, nil
}

// ImportConnections 合并导入：同 ID 覆盖、同组重名自动加后缀，不会静默丢数据
func ImportConnections(in []ConnectionConfig) (ImportSummary, error) {
	var summary ImportSummary
	if len(in) == 0 {
		summary.Messages = append(summary.Messages, "文件里没有连接配置")
		return summary, nil
	}

	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return summary, err
	}

	indexByID := make(map[string]int, len(store.Connections))
	for i, c := range store.Connections {
		indexByID[c.ID] = i
	}
	nameKey := func(group, name string) string {
		return strings.ToLower(strings.TrimSpace(group)) + "\x00" + strings.ToLower(strings.TrimSpace(name))
	}
	usedNames := make(map[string]struct{}, len(store.Connections))
	for _, c := range store.Connections {
		usedNames[nameKey(c.Group, c.Name)] = struct{}{}
	}

	now := time.Now()
	for _, cfg := range in {
		cfg.Name = strings.TrimSpace(cfg.Name)
		cfg.Group = strings.TrimSpace(cfg.Group)
		if cfg.Name == "" {
			summary.Skipped++
			summary.Messages = append(summary.Messages, "跳过一条没有名称的连接")
			continue
		}
		if !cfg.IsCluster && strings.TrimSpace(cfg.Host) == "" {
			summary.Skipped++
			summary.Messages = append(summary.Messages, "跳过「"+cfg.Name+"」：缺少地址")
			continue
		}
		if cfg.Port == 0 {
			cfg.Port = 6379
		}

		if idx, exists := indexByID[cfg.ID]; exists && cfg.ID != "" {
			// 同 ID：视为同一台机器上的同一连接，直接覆盖
			original := store.Connections[idx]
			cfg.CreatedAt = original.CreatedAt
			cfg.SortOrder = original.SortOrder
			cfg.UpdatedAt = now
			// 先把自己原来的名字从占用表里移除，否则会和自己冲突、被加后缀
			delete(usedNames, nameKey(original.Group, original.Name))
			cfg.Name = uniqueName(usedNames, nameKey, cfg.Group, cfg.Name)
			usedNames[nameKey(cfg.Group, cfg.Name)] = struct{}{}
			store.Connections[idx] = cfg
			summary.Updated++
			summary.UpdatedIDs = append(summary.UpdatedIDs, cfg.ID)
			continue
		}

		finalName := uniqueName(usedNames, nameKey, cfg.Group, cfg.Name)
		if finalName != cfg.Name {
			summary.Renamed++
			summary.Messages = append(summary.Messages, "重命名「"+cfg.Name+"」为「"+finalName+"」：分组内已有同名连接")
			cfg.Name = finalName
		}
		usedNames[nameKey(cfg.Group, cfg.Name)] = struct{}{}
		cfg.ID = uuid.New().String()
		cfg.CreatedAt = now
		cfg.UpdatedAt = now
		cfg.SortOrder = len(store.Connections)
		store.Connections = append(store.Connections, cfg)
		summary.Added++
	}

	if summary.Added == 0 && summary.Updated == 0 {
		return summary, nil
	}
	return summary, saveStore(store)
}

// uniqueName 在分组内找一个没被占用的名字：name → name (2) → name (3)
func uniqueName(used map[string]struct{}, key func(group, name string) string, group, name string) string {
	if _, taken := used[key(group, name)]; !taken {
		return name
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)", name, i)
		if _, taken := used[key(group, candidate)]; !taken {
			return candidate
		}
	}
	return name + " (imported)"
}
