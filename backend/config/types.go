package config

import "time"

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	SortOrder        int        `json:"sort_order"`
	Group            string     `json:"group,omitempty"`
	Host             string     `json:"host"`
	Port             int        `json:"port"`
	Password         string     `json:"password"`
	Username         string     `json:"username,omitempty"`    // Redis 6+ ACL 用户名，留空则按旧式 AUTH <password>
	TLSEnabled       bool       `json:"tls_enabled"`           // 使用 TLS/SSL 连接（rediss://）
	TLSSkipVerify    bool       `json:"tls_skip_verify"`       // 跳过证书校验，自签证书时使用
	TLSCAFile        string     `json:"tls_ca_file,omitempty"` // 自定义 CA 证书路径
	ReadOnly         bool       `json:"read_only"`             // 保护模式：阻止所有写操作
	DB               int        `json:"db"`
	IsCluster        bool       `json:"is_cluster"`
	AllowClusterScan bool       `json:"allow_cluster_scan"`
	ClusterAddrs     []string   `json:"cluster_addrs,omitempty"`
	ProxyEnabled     bool       `json:"proxy_enabled"`
	ProxyURL         string     `json:"proxy_url,omitempty"`
	IconColor        string     `json:"icon_color,omitempty"`
	SSHEnabled       bool       `json:"ssh_enabled"`
	SSH              *SSHConfig `json:"ssh,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ConnectionOrderItem struct {
	ID        string `json:"id"`
	Group     string `json:"group,omitempty"`
	SortOrder int    `json:"sort_order"`
}

// SSHConfig SSH 配置
type SSHConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	User           string `json:"user"`
	Password       string `json:"password"`
	PrivateKeyPath string `json:"private_key_path,omitempty"`
	Passphrase     string `json:"passphrase,omitempty"`
}

// AppSettings 全局应用设置
type AppSettings struct {
	KeyScanCount       int64  `json:"key_scan_count"`       // 每次扫描 key 的数量，默认 20
	HashLoadCount      int64  `json:"hash_load_count"`      // Hash 每次加载 field 数，默认 20
	ListLoadCount      int64  `json:"list_load_count"`      // List 每次加载条数，默认 20
	SetLoadCount       int64  `json:"set_load_count"`       // Set 每次加载成员数，默认 20
	ZSetLoadCount      int64  `json:"zset_load_count"`      // ZSet 每次加载条数，默认 20
	StreamLoadCount    int64  `json:"stream_load_count"`    // Stream 每次加载条数，默认 20
	SearchHistoryLimit int    `json:"search_history_limit"` // 保留最近查询历史条数，默认 10
	KeyDisplayMode     string `json:"key_display_mode"`     // key 展示模式: tree | flat
	FontSizeLevel      string `json:"font_size_level"`      // 字号档位: small | medium | large
	WatermarkEnabled   bool   `json:"watermark_enabled"`    // 是否开启水印
	WatermarkText      string `json:"watermark_text"`       // 水印文字
	WatermarkSize      int    `json:"watermark_size"`       // 水印字号
	WatermarkAngle     int    `json:"watermark_angle"`      // 水印角度
	WatermarkOpacity   int    `json:"watermark_opacity"`    // 水印透明度 0-100
	WatermarkDensity   int    `json:"watermark_density"`    // 水印密度 1-5
	Language           string `json:"language"`             // 界面语言，默认 zh
	LocalAPIEnabled    bool   `json:"local_api_enabled"`    // 本地数据提取 API 服务开关，默认 true
	LocalAPIPort       int    `json:"local_api_port"`       // 本地 API 服务端口，默认 64201，仅监听 127.0.0.1
	// 应用包更新代理：仅用于访问 GitHub 与下载安装包，不影响数据库连接
	ProxyEnabled bool   `json:"proxy_enabled"` // 代理开关
	ProxyURL     string `json:"proxy_url"`     // http://host:port 或 socks5://host:port
}

const (
	// LocalAPIPortProd 正式版默认的本地 API 端口
	LocalAPIPortProd = 64201
	// LocalAPIPortDev 开发版默认端口，避免与同时运行的正式版抢占同一端口
	LocalAPIPortDev = 64202
)

var (
	defaultLocalAPIPort = LocalAPIPortProd
	devBuild            bool
)

// SetDevBuild 由 main 在启动时按构建类型调用，决定默认端口与端口互让规则
func SetDevBuild(isDev bool) {
	devBuild = isDev
	if isDev {
		defaultLocalAPIPort = LocalAPIPortDev
	} else {
		defaultLocalAPIPort = LocalAPIPortProd
	}
}

// IsDevBuild 当前是否为开发构建
func IsDevBuild() bool {
	return devBuild
}

// DefaultLocalAPIPort 返回当前构建的默认端口
func DefaultLocalAPIPort() int {
	return defaultLocalAPIPort
}

// EffectiveLocalAPIPort 计算实际监听端口：
// 未配置时用默认值；64201 归正式版、64202 归开发版，
// 两个构建互相让开，避免同时运行时抢占同一端口。
func EffectiveLocalAPIPort(configured int) int {
	if configured <= 0 {
		configured = defaultLocalAPIPort
	}
	if devBuild && configured == LocalAPIPortProd {
		return LocalAPIPortDev
	}
	if !devBuild && configured == LocalAPIPortDev {
		return LocalAPIPortProd
	}
	return configured
}

// DefaultSettings 返回默认设置
func DefaultSettings() AppSettings {
	return AppSettings{
		KeyScanCount:       20,
		HashLoadCount:      20,
		ListLoadCount:      20,
		SetLoadCount:       20,
		ZSetLoadCount:      20,
		StreamLoadCount:    20,
		SearchHistoryLimit: 10,
		KeyDisplayMode:     "tree",
		FontSizeLevel:      "small",
		WatermarkEnabled:   false,
		WatermarkText:      "",
		WatermarkSize:      16,
		WatermarkAngle:     -22,
		WatermarkOpacity:   12,
		WatermarkDensity:   3,
		Language:           "zh",
		LocalAPIEnabled:    true,
		LocalAPIPort:       defaultLocalAPIPort,
		ProxyEnabled:       false,
		ProxyURL:           "",
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
	Name  string `json:"name"`
	Type  string `json:"type"`
	TTL   int64  `json:"ttl"` // -1=永久，-2=不存在，>0=剩余秒数
	Count int64  `json:"count"`
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
	Key          string            `json:"key"`
	Type         string            `json:"type"`
	TTL          int64             `json:"ttl"`
	StringVal    string            `json:"string_val,omitempty"`
	HashVal      map[string]string `json:"hash_val,omitempty"`
	ListVal      []string          `json:"list_val,omitempty"`
	SetVal       []string          `json:"set_val,omitempty"`
	ZSetVal      []ZSetMember      `json:"zset_val,omitempty"`
	StreamVal    []StreamEntry     `json:"stream_val,omitempty"`
	HasMore      bool              `json:"has_more"`                 // 是否还有更多数据
	NextCursor   uint64            `json:"next_cursor"`              // Hash/Set 下一页 cursor
	NextOffset   int               `json:"next_offset"`              // List/ZSet 下一页 offset
	NextStreamID string            `json:"next_stream_id,omitempty"` // Stream 下一页的排他起始 ID
	TotalCount   int64             `json:"total_count"`              // 总元素数（-1 表示未知）
	Truncated    bool              `json:"truncated,omitempty"`      // string 值超限被截断
}

// CreateKeyRequest 新建 key 请求
type CreateKeyRequest struct {
	Key         string  `json:"key"`
	Type        string  `json:"type"`
	TTL         int64   `json:"ttl"`
	StringValue string  `json:"string_value,omitempty"`
	Field       string  `json:"field,omitempty"`
	Value       string  `json:"value,omitempty"`
	ListValue   string  `json:"list_value,omitempty"`
	Member      string  `json:"member,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

// ScanResult key 扫描结果（支持分页）
type ScanResult struct {
	Keys       []RedisKey `json:"keys"`
	NextCursor uint64     `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
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
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	Disconnected bool   `json:"disconnected,omitempty"`
}

// RedisConnectionOverview 当前连接概览信息
type RedisConnectionOverview struct {
	ConnID           string `json:"conn_id"`
	ConnName         string `json:"conn_name"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	CurrentDB        int    `json:"current_db"`
	IsCluster        bool   `json:"is_cluster"`
	RedisVersion     string `json:"redis_version"`
	Role             string `json:"role"`
	ConnectedClients int64  `json:"connected_clients"`
	InstantOpsPerSec int64  `json:"instant_ops_per_sec"`
	TotalKeys        int64  `json:"total_keys"`
	UsedMemory       string `json:"used_memory"`
	UsedMemoryBytes  int64  `json:"used_memory_bytes"`
	UptimeDays       int64  `json:"uptime_days"`
	UptimeHuman      string `json:"uptime_human"`
	// 以下字段供实时监控面板使用
	MaxMemoryBytes int64   `json:"max_memory_bytes"`
	KeyspaceHits   int64   `json:"keyspace_hits"`
	KeyspaceMisses int64   `json:"keyspace_misses"`
	EvictedKeys    int64   `json:"evicted_keys"`
	BlockedClients int64   `json:"blocked_clients"`
	HitRate        float64 `json:"hit_rate"`
}

// MemoryKeyInfo 内存体检中的单个 key 明细
type MemoryKeyInfo struct {
	Key  string `json:"key"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	// Len 元素数量（string 为字节长度），仅服务端不支持内存查询时使用
	Len int64 `json:"len,omitempty"`
	TTL int64 `json:"ttl"`
}

// Weight 用于大 key 排序：有元素数时按元素数，否则按字节数
func (k MemoryKeyInfo) Weight() int64 {
	if k.Len > 0 {
		return k.Len
	}
	return k.Size
}

// MemoryPrefixStat 按 key 前缀聚合的内存占用
type MemoryPrefixStat struct {
	Prefix string `json:"prefix"`
	Keys   int64  `json:"keys"`
	Bytes  int64  `json:"bytes"`
}

// MemoryTypeStat 按数据类型聚合的内存占用
type MemoryTypeStat struct {
	Type  string `json:"type"`
	Keys  int64  `json:"keys"`
	Bytes int64  `json:"bytes"`
}

// MemoryReport 内存体检结果
type MemoryReport struct {
	ConnID          string `json:"conn_id"`
	DB              int    `json:"db"`
	Pattern         string `json:"pattern"`
	ScannedKeys     int64  `json:"scanned_keys"`
	TotalKeys       int64  `json:"total_keys"`
	TotalBytes      int64  `json:"total_bytes"`
	AvgKeyBytes     int64  `json:"avg_key_bytes"`
	NoTTLKeys       int64  `json:"no_ttl_keys"`
	Truncated       bool   `json:"truncated"`
	SizeUnavailable bool   `json:"size_unavailable"`
	// SizeSource 大小来源：memory（MEMORY USAGE）/ debug（DEBUG OBJECT）/ length（元素数量）
	SizeSource string             `json:"size_source,omitempty"`
	ElapsedMs  int64              `json:"elapsed_ms"`
	BigKeys    []MemoryKeyInfo    `json:"big_keys"`
	Prefixes   []MemoryPrefixStat `json:"prefixes"`
	Types      []MemoryTypeStat   `json:"types"`
}

// ExportedKey 导出的单个 key（结构化，可用于导出文件或跨实例复制）
type ExportedKey struct {
	Key    string            `json:"key"`
	Type   string            `json:"type"`
	TTL    int64             `json:"ttl"`
	String string            `json:"string,omitempty"`
	Hash   map[string]string `json:"hash,omitempty"`
	List   []string          `json:"list,omitempty"`
	Set    []string          `json:"set,omitempty"`
	ZSet   []ZSetMember      `json:"zset,omitempty"`
	Stream []StreamEntry     `json:"stream,omitempty"`
}

// SlowLogEntry 慢查询记录
type SlowLogEntry struct {
	ID         int64  `json:"id"`
	Timestamp  int64  `json:"timestamp"`
	DurationUs int64  `json:"duration_us"`
	Command    string `json:"command"`
	ClientAddr string `json:"client_addr,omitempty"`
	ClientName string `json:"client_name,omitempty"`
	Node       string `json:"node,omitempty"`
}

// BatchKeyResult 批量操作结果
type BatchKeyResult struct {
	Success  bool   `json:"success"`
	Affected int    `json:"affected"`
	Failed   int    `json:"failed"`
	Message  string `json:"message,omitempty"`
}

// RedisConsoleResult Redis Console 执行结果
type RedisConsoleResult struct {
	Success   bool   `json:"success"`
	Command   string `json:"command"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	ElapsedMs int64  `json:"elapsed_ms"`
	// Blocked 保护模式下命令被拦截，前端可确认后强制执行
	Blocked bool `json:"blocked,omitempty"`
}
