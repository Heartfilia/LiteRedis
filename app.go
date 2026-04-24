package main

import (
	"context"
	"time"

	"LiteRedis/backend/config"
	redisbackend "LiteRedis/backend/redis"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx             context.Context
	manager         *redisbackend.ClientManager
	initWindowState config.WindowState
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		manager: redisbackend.NewClientManager(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 恢复上次窗口位置（X=-1 表示首次启动，让系统居中）
	if a.initWindowState.X != -1 {
		wailsruntime.WindowSetPosition(ctx, a.initWindowState.X, a.initWindowState.Y)
	}
}

// shutdown is called when the app exits
func (a *App) shutdown(ctx context.Context) {
	// 保存窗口尺寸和位置
	w, h := wailsruntime.WindowGetSize(ctx)
	x, y := wailsruntime.WindowGetPosition(ctx)
	_ = config.SaveWindowState(config.WindowState{X: x, Y: y, Width: w, Height: h})

	a.manager.DisconnectAll()
}

// ============================================================
// 连接管理
// ============================================================

// ListConnections 返回所有连接配置
func (a *App) ListConnections() []config.ConnectionConfig {
	conns, err := config.ListConnections()
	if err != nil {
		return []config.ConnectionConfig{}
	}
	return conns
}

// SaveConnection 新建或更新连接配置
func (a *App) SaveConnection(cfg config.ConnectionConfig) config.OperationResult {
	saved, err := config.SaveConnection(cfg)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	_ = saved
	return config.OperationResult{Success: true}
}

// DeleteConnection 删除连接配置
func (a *App) DeleteConnection(id string) config.OperationResult {
	// 先断开连接
	a.manager.Disconnect(id)
	if err := config.DeleteConnection(id); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// TestConnection 测试连通性（不持久化）
func (a *App) TestConnection(cfg config.ConnectionConfig) config.OperationResult {
	if err := redisbackend.TestConnection(cfg); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true, Message: "连接成功"}
}

// ConnectResult 连接结果，包含初始 DB
type ConnectResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	InitDB  int    `json:"init_db"`
}

// Connect 建立活跃连接，返回连接配置中预设的 DB
func (a *App) Connect(id string) ConnectResult {
	cfg, err := config.GetConnection(id)
	if err != nil || cfg == nil {
		return ConnectResult{Success: false, Message: "连接配置不存在"}
	}
	if err := a.manager.Connect(*cfg); err != nil {
		return ConnectResult{Success: false, Message: err.Error()}
	}
	return ConnectResult{Success: true, InitDB: cfg.DB}
}

// Disconnect 断开连接
func (a *App) Disconnect(id string) config.OperationResult {
	a.manager.Disconnect(id)
	return config.OperationResult{Success: true}
}

// IsConnected 查询连接状态
func (a *App) IsConnected(id string) bool {
	return a.manager.IsConnected(id)
}

// SelectDB 切换数据库
func (a *App) SelectDB(id string, db int) config.OperationResult {
	if err := a.manager.SelectDB(id, db); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// ============================================================
// Key 操作
// ============================================================

// ScanKeys 扫描 key（SCAN + Pipeline 批量 TYPE/TTL），支持 cursor 分页。count<=0 时使用设置中的值，cursor=0 表示第一页。
func (a *App) ScanKeys(connID string, pattern string, count int64, cursor uint64) (config.ScanResult, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.ScanResult{}, err
	}
	if count <= 0 {
		settings, _ := config.GetSettings()
		count = settings.KeyScanCount
		if count <= 0 {
			count = 100
		}
	}
	return redisbackend.ScanKeys(a.ctx, client, pattern, count, cursor)
}

// GetKeyInfo 获取单个 key 元信息
func (a *App) GetKeyInfo(connID string, key string) (config.RedisKey, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.RedisKey{}, err
	}
	return redisbackend.GetKeyInfo(a.ctx, client, key)
}

// DeleteKey 删除 key
func (a *App) DeleteKey(connID string, key string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.DeleteKey(a.ctx, client, key); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// RenameKey 重命名 key
func (a *App) RenameKey(connID, oldKey, newKey string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.RenameKey(a.ctx, client, oldKey, newKey); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// SetTTL 设置 key TTL
func (a *App) SetTTL(connID, key string, ttlSec int64) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.SetTTL(a.ctx, client, key, ttlSec); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// DBSize 获取当前 DB 的 key 总数
func (a *App) DBSize(connID string) int64 {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return 0
	}
	size, _ := redisbackend.DBSize(a.ctx, client)
	return size
}

// ============================================================
// Value CRUD
// ============================================================

// GetValue 读取 Value（支持 cursor/offset 分页）。cursor=0, offset=0 表示第一页。
func (a *App) GetValue(connID, key string, cursor uint64, offset int) (config.KeyValue, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.KeyValue{}, err
	}
	settings, _ := config.GetSettings()
	// 每次读取用独立超时 context，防止大 key 阻塞整个应用
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	return redisbackend.GetValue(ctx, client, key, settings, cursor, offset)
}

// SearchValue 按 pattern 搜索 key 内成员（Hash/Set/ZSet 使用 Redis glob，List 使用子串匹配）
func (a *App) SearchValue(connID, key, keyType, pattern string) (config.KeyValue, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.KeyValue{}, err
	}
	settings, _ := config.GetSettings()
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	return redisbackend.SearchValue(ctx, client, key, keyType, pattern, settings)
}

// SetString 设置 string
func (a *App) SetString(connID, key, value string, ttlSec int64) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.SetString(a.ctx, client, key, value, ttlSec); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// HSet 设置 hash field
func (a *App) HSet(connID, key, field, value string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.HSet(a.ctx, client, key, field, value); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// HDel 删除 hash field
func (a *App) HDel(connID, key, field string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.HDel(a.ctx, client, key, field); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// LPush 向 list 头部插入
func (a *App) LPush(connID, key, value string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.LPush(a.ctx, client, key, value); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// RPush 向 list 尾部插入
func (a *App) RPush(connID, key, value string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.RPush(a.ctx, client, key, value); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// LSet 设置 list 指定索引的值
func (a *App) LSet(connID, key string, index int64, value string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.LSet(a.ctx, client, key, index, value); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// LRem 删除 list 中匹配的元素
func (a *App) LRem(connID, key string, count int64, value string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.LRem(a.ctx, client, key, count, value); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// SAdd 向 set 添加成员
func (a *App) SAdd(connID, key, member string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.SAdd(a.ctx, client, key, member); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// SRem 从 set 删除成员
func (a *App) SRem(connID, key, member string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.SRem(a.ctx, client, key, member); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// ZAdd 向 zset 添加成员
func (a *App) ZAdd(connID, key, member string, score float64) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.ZAdd(a.ctx, client, key, member, score); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// ZRem 从 zset 删除成员
func (a *App) ZRem(connID, key, member string) config.OperationResult {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.ZRem(a.ctx, client, key, member); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// ============================================================
// 设置管理
// ============================================================

// GetSettings 获取全局设置
func (a *App) GetSettings() config.AppSettings {
	s, err := config.GetSettings()
	if err != nil {
		return config.DefaultSettings()
	}
	return s
}

// SaveSettings 保存全局设置
func (a *App) SaveSettings(s config.AppSettings) config.OperationResult {
	if err := config.SaveSettings(s); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}
