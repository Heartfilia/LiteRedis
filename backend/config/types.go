package config

import "time"

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Group        string     `json:"group,omitempty"`
	Host         string     `json:"host"`
	Port         int        `json:"port"`
	Password     string     `json:"password"`
	DB           int        `json:"db"`
	IsCluster    bool       `json:"is_cluster"`
	ClusterAddrs []string   `json:"cluster_addrs,omitempty"`
	SSHEnabled   bool       `json:"ssh_enabled"`
	SSH          *SSHConfig `json:"ssh,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// SSHConfig SSH 配置
type SSHConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// AppSettings 全局应用设置
type AppSettings struct {
	KeyScanCount    int64 `json:"key_scan_count"`    // 每次扫描 key 的数量，默认 20
	HashLoadCount   int64 `json:"hash_load_count"`   // Hash 每次加载 field 数，默认 20
	ListLoadCount   int64 `json:"list_load_count"`   // List 每次加载条数，默认 20
	SetLoadCount    int64 `json:"set_load_count"`    // Set 每次加载成员数，默认 20
	ZSetLoadCount   int64 `json:"zset_load_count"`   // ZSet 每次加载条数，默认 20
	StreamLoadCount int64 `json:"stream_load_count"` // Stream 每次加载条数，默认 20
}

// DefaultSettings 返回默认设置
func DefaultSettings() AppSettings {
	return AppSettings{
		KeyScanCount:    20,
		HashLoadCount:   20,
		ListLoadCount:   20,
		SetLoadCount:    20,
		ZSetLoadCount:   20,
		StreamLoadCount: 20,
	}
}

// ConfigStore 配置文件根结构
type ConfigStore struct {
	Version     int                `json:"version"`
	Connections []ConnectionConfig `json:"connections"`
	Settings    AppSettings        `json:"settings"`
}

// RedisKey key 元信息
type RedisKey struct {
	Name string `json:"name"`
	Type string `json:"type"`
	TTL  int64  `json:"ttl"` // -1=永久，-2=不存在，>0=剩余秒数
}

// KeyNode 树节点（:折叠）
type KeyNode struct {
	Label    string     `json:"label"`
	FullPath string     `json:"full_path"`
	IsLeaf   bool       `json:"is_leaf"`
	KeyType  string     `json:"key_type,omitempty"`
	TTL      int64      `json:"ttl,omitempty"`
	Children []*KeyNode `json:"children,omitempty"`
	Count    int        `json:"count"`
}

// KeyValue Value（按类型按需填充）
type KeyValue struct {
	Key       string            `json:"key"`
	Type      string            `json:"type"`
	TTL       int64             `json:"ttl"`
	StringVal string            `json:"string_val,omitempty"`
	HashVal   map[string]string `json:"hash_val,omitempty"`
	ListVal   []string          `json:"list_val,omitempty"`
	SetVal    []string          `json:"set_val,omitempty"`
	ZSetVal   []ZSetMember      `json:"zset_val,omitempty"`
	StreamVal []StreamEntry     `json:"stream_val,omitempty"`
}

// ZSetMember zset 成员
type ZSetMember struct {
	Member string  `json:"member"`
	Score  float64 `json:"score"`
}

// StreamEntry stream 条目
type StreamEntry struct {
	ID     string            `json:"id"`
	Fields map[string]string `json:"fields"`
}

// OperationResult 通用操作结果
type OperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
