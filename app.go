package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"LiteRedis/backend/config"
	"LiteRedis/backend/localapi"
	redisbackend "LiteRedis/backend/redis"
	sshbackend "LiteRedis/backend/ssh"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx       context.Context
	manager   *redisbackend.ClientManager
	registry  *localapi.KeyRegistry
	apiServer *localapi.Server

	pubsubMu        sync.Mutex
	pubsub          map[string]*pubSubSession
	initWindowState config.WindowState

	memoryScanMu     sync.Mutex
	memoryScanCancel context.CancelFunc
}

// MemoryScanProgress 内存体检进度，通过 memory:progress 事件推送
type MemoryScanProgress struct {
	Scanned int64   `json:"scanned"`
	Total   int64   `json:"total"`
	Percent float64 `json:"percent"`
}

type VersionInfo struct {
	Version    string `json:"version"`
	Latest     string `json:"latest"`
	NeedUpdate bool   `json:"need_update"`
	ReleaseURL string `json:"release_url,omitempty"`
	CheckedAt  string `json:"checked_at,omitempty"`
	Error      string `json:"error,omitempty"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	registry := localapi.NewKeyRegistry()
	return &App{
		manager:   redisbackend.NewClientManager(),
		registry:  registry,
		apiServer: localapi.NewServer(registry),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_ = cleanupStaleUpdateDirs(os.TempDir(), time.Now().Add(-updateTempMaxAge))
	// 启动本地数据提取 API（默认开启，失败不影响主功能）
	if settings, err := config.GetSettings(); err == nil {
		a.configureLocalAPI(settings)
		if settings.LocalAPIEnabled {
			port := config.EffectiveLocalAPIPort(settings.LocalAPIPort)
			if err := a.apiServer.Start(port); err != nil {
				config.AppendDebugLog("[localapi] start failed: %v", err)
			}
		}
	}
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

	a.stopAllPubSub()
	a.manager.DisconnectAll()
	a.apiServer.Stop()
}

// GetAppVersion 返回当前构建版本
func (a *App) GetAppVersion() string {
	return AppVersion
}

// ============================================================
// 连接管理
// ============================================================

// ListConnections 返回所有连接配置
func (a *App) ListConnections() ([]config.ConnectionConfig, error) {
	return config.ListConnections()
}

// SaveConnection 新建或更新连接配置
func (a *App) SaveConnection(cfg config.ConnectionConfig) config.OperationResult {
	saved, err := config.SaveConnection(cfg)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if a.manager.ApplyConnectionConfig(saved) {
		// 传输层变更导致断开，已发布快照一并清除
		a.registry.UnpublishConn(saved.ID)
		return config.OperationResult{
			Success:      true,
			Disconnected: true,
			Message:      "connection settings changed; reconnect required",
		}
	}
	return config.OperationResult{Success: true}
}

func (a *App) ReorderConnections(items []config.ConnectionOrderItem) config.OperationResult {
	if err := config.ReorderConnections(items); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// DeleteConnection 删除连接配置
func (a *App) DeleteConnection(id string) config.OperationResult {
	// 先断开连接
	a.manager.Disconnect(id)
	a.registry.UnpublishConn(id)
	if err := config.DeleteConnection(id); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// TestConnection 测试连通性（不持久化）
func (a *App) TestConnection(cfg config.ConnectionConfig) config.OperationResult {
	if err := redisbackend.TestConnection(cfg); err != nil {
		return config.OperationResult{Success: false, Message: err.Error() + "\nDebug log: " + config.DebugLogPath()}
	}
	return config.OperationResult{Success: true, Message: "连接成功\nDebug log: " + config.DebugLogPath()}
}

// ForgetSSHHostFingerprint 删除指定 SSH 主机的已记录指纹（服务器换钥后重置用）
func (a *App) ForgetSSHHostFingerprint(host string, port int) config.OperationResult {
	if err := sshbackend.ForgetKnownHost(host, port); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true, Message: "已重置 SSH 主机指纹，下次连接将重新记录"}
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
	if err != nil {
		return ConnectResult{Success: false, Message: err.Error()}
	}
	if cfg == nil {
		return ConnectResult{Success: false, Message: "连接配置不存在"}
	}
	if err := a.manager.Connect(*cfg); err != nil {
		return ConnectResult{Success: false, Message: err.Error() + "\nDebug log: " + config.DebugLogPath()}
	}
	return ConnectResult{Success: true, InitDB: cfg.DB}
}

// Disconnect 断开连接
func (a *App) Disconnect(id string) config.OperationResult {
	a.stopPubSub(id)
	a.manager.Disconnect(id)
	a.registry.UnpublishConn(id)
	return config.OperationResult{Success: true}
}

// IsConnected 查询连接状态
func (a *App) IsConnected(id string) bool {
	return a.manager.IsConnected(id)
}

// PingConnection 检测连接是否仍然可用
func (a *App) PingConnection(id string) config.OperationResult {
	if err := a.manager.Ping(id); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// SelectDB 切换数据库
func (a *App) SelectDB(id string, db int) config.OperationResult {
	if err := a.manager.SelectDB(id, db); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	// DB 上下文变化，旧 DB 的快照不再有效
	a.registry.UnpublishConn(id)
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
	cfg, cfgErr := config.GetConnection(connID)
	if cfgErr == nil && cfg != nil && cfg.IsCluster && !cfg.AllowClusterScan {
		normalized := strings.TrimSpace(pattern)
		if normalized == "" || normalized == "*" || strings.ContainsAny(normalized, "*?[") {
			return config.ScanResult{}, nil
		}
	}
	if count <= 0 {
		settings, err := config.GetSettings()
		if err != nil {
			return config.ScanResult{}, err
		}
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
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.DeleteKey(a.ctx, client, key); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	a.registry.Unpublish(connID, key)
	return config.OperationResult{Success: true}
}

// RenameKey 重命名 key
func (a *App) RenameKey(connID, oldKey, newKey string) config.OperationResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.RenameKey(a.ctx, client, oldKey, newKey); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	a.registry.Unpublish(connID, oldKey)
	return config.OperationResult{Success: true}
}

// SetTTL 设置 key TTL
func (a *App) SetTTL(connID, key string, ttlSec int64) config.OperationResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
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
func (a *App) DBSize(connID string) (int64, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return 0, err
	}
	return redisbackend.DBSize(a.ctx, client)
}

// GetConnectionOverview 获取当前连接概览信息
func (a *App) GetConnectionOverview(connID string) (config.RedisConnectionOverview, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.RedisConnectionOverview{}, err
	}
	cfg, currentDB, err := a.manager.GetConnectionState(connID)
	if err != nil {
		return config.RedisConnectionOverview{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
	defer cancel()
	return redisbackend.GetConnectionOverview(ctx, client, cfg, currentDB)
}

// ExecuteRedisCommand 在当前连接上执行 Redis 命令
// GetConnectionDB 返回连接当前选中的 DB（集群模式固定为 0），供界面展示扫描范围
func (a *App) GetConnectionDB(connID string) (int, error) {
	_, db, err := a.manager.GetConnectionState(connID)
	if err != nil {
		return 0, err
	}
	return db, nil
}

// errReadOnly 保护模式下拒绝写操作的统一错误
var errReadOnly = errors.New("当前连接处于保护模式，已阻止写操作")

// invalidateListCache 列表内容变化后让分页缓存失效
func (a *App) invalidateListCache(connID, key string) {
	_, db, err := a.manager.GetConnectionState(connID)
	if err != nil {
		return
	}
	redisbackend.InvalidateListChunks(connID+"|"+strconv.Itoa(db), key)
}

// ensureWritable 在所有写操作入口校验连接是否处于保护模式。
// 只靠前端置灰不够可靠，后端必须兜底，避免误调用真的改到数据。
func (a *App) ensureWritable(connID string) error {
	cfg, _, err := a.manager.GetConnectionState(connID)
	if err != nil {
		return err
	}
	if cfg.ReadOnly {
		return errReadOnly
	}
	return nil
}

// BatchDeleteKeys 批量删除 key
func (a *App) BatchDeleteKeys(connID string, keys []string) config.BatchKeyResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.BatchKeyResult{Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.BatchKeyResult{Message: err.Error()}
	}
	if len(keys) == 0 {
		return config.BatchKeyResult{Success: true}
	}
	affected, err := redisbackend.DeleteKeys(a.ctx, client, keys)
	for _, key := range keys {
		a.registry.Unpublish(connID, key)
	}
	if err != nil {
		return config.BatchKeyResult{Message: err.Error(), Affected: affected, Failed: len(keys) - affected}
	}
	return config.BatchKeyResult{Success: true, Affected: affected, Failed: len(keys) - affected}
}

// BatchSetKeysTTL 批量设置 TTL，ttlSec < 0 表示永久
func (a *App) BatchSetKeysTTL(connID string, keys []string, ttlSec int64) config.BatchKeyResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.BatchKeyResult{Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.BatchKeyResult{Message: err.Error()}
	}
	if len(keys) == 0 {
		return config.BatchKeyResult{Success: true}
	}
	affected, err := redisbackend.SetKeysTTL(a.ctx, client, keys, ttlSec)
	if err != nil {
		return config.BatchKeyResult{Message: err.Error(), Affected: affected, Failed: len(keys) - affected}
	}
	return config.BatchKeyResult{Success: true, Affected: affected, Failed: len(keys) - affected}
}

// BatchRenamePrefix 批量替换选中 key 的前缀
func (a *App) BatchRenamePrefix(connID string, keys []string, oldPrefix, newPrefix string) config.BatchKeyResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.BatchKeyResult{Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.BatchKeyResult{Message: err.Error()}
	}
	if len(keys) == 0 || strings.TrimSpace(oldPrefix) == "" {
		return config.BatchKeyResult{Success: true}
	}
	renamed, err := redisbackend.RenameKeysPrefix(a.ctx, client, keys, oldPrefix, newPrefix)
	if err != nil {
		return config.BatchKeyResult{Message: err.Error(), Affected: renamed, Failed: len(keys) - renamed}
	}
	return config.BatchKeyResult{Success: true, Affected: renamed, Failed: len(keys) - renamed}
}

// GetDatabases 返回连接的 DB 数量（集群为 1，读取失败回退 16）
func (a *App) GetDatabases(connID string) int {
	cfg, _, err := a.manager.GetConnectionState(connID)
	if err != nil {
		return 0
	}
	if cfg.IsCluster {
		return 1
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return 0
	}
	return redisbackend.GetDatabaseCount(a.ctx, client)
}

// ExportKeysToFile 把选中的 key 导出到文件，返回实际写入路径（用户取消时为空）
func (a *App) ExportKeysToFile(connID string, keys []string, format string) (string, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return "", err
	}
	if len(keys) == 0 {
		return "", errors.New("请先选择要导出的 key")
	}
	if len(keys) > redisbackend.ExportMaxKeys {
		return "", fmt.Errorf("单次最多导出 %d 个 key", redisbackend.ExportMaxKeys)
	}

	entries := make([]config.ExportedKey, 0, len(keys))
	for _, key := range keys {
		entry, err := redisbackend.ReadKeyForExport(a.ctx, client, key)
		if err != nil {
			return "", fmt.Errorf("%s: %w", key, err)
		}
		entries = append(entries, entry)
	}

	// 文件名带时间戳，连续导出两次不会互相覆盖
	stamp := time.Now().Format("20060102-150405")
	var content []byte
	filters := []wailsruntime.FileFilter{}
	defaultName := "literedis-export-" + stamp + ".json"
	if strings.EqualFold(format, "commands") {
		content = []byte(redisbackend.FormatExportCommands(entries))
		defaultName = "literedis-commands-" + stamp + ".txt"
		filters = append(filters, wailsruntime.FileFilter{DisplayName: "Redis commands", Pattern: "*.txt"})
	} else {
		content, err = redisbackend.FormatExportJSON(entries)
		if err != nil {
			return "", err
		}
		filters = append(filters, wailsruntime.FileFilter{DisplayName: "JSON", Pattern: "*.json"})
	}

	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出 Key",
		DefaultFilename: defaultName,
		Filters:         filters,
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		return "", err
	}
	return path, nil
}

// ImportKeysFromFile 从导出的 JSON 文件导入 key（写入当前 DB）
func (a *App) ImportKeysFromFile(connID string, overwrite bool) (config.BatchKeyResult, error) {
	if err := a.ensureWritable(connID); err != nil {
		return config.BatchKeyResult{Message: err.Error()}, nil
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.BatchKeyResult{Message: err.Error()}, nil
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入 Key",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "JSON", Pattern: "*.json"},
		},
	})
	if err != nil {
		return config.BatchKeyResult{Message: err.Error()}, nil
	}
	if strings.TrimSpace(path) == "" {
		return config.BatchKeyResult{Message: "已取消"}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return config.BatchKeyResult{Message: err.Error()}, nil
	}
	if info.Size() > 64<<20 {
		return config.BatchKeyResult{Message: "文件超过 64MB，请拆分后再导入"}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config.BatchKeyResult{Message: err.Error()}, nil
	}

	var entries []config.ExportedKey
	if err := json.Unmarshal(data, &entries); err != nil {
		return config.BatchKeyResult{Message: "不是有效的导出文件: " + err.Error()}, nil
	}
	if len(entries) == 0 {
		return config.BatchKeyResult{Message: "文件中没有可导入的 key"}, nil
	}
	if len(entries) > redisbackend.ExportMaxKeys {
		return config.BatchKeyResult{Message: fmt.Sprintf("单次最多导入 %d 个 key", redisbackend.ExportMaxKeys)}, nil
	}

	imported, skipped, failed := 0, 0, 0
	var firstErr error
	for _, entry := range entries {
		written, err := redisbackend.WriteExportedKey(a.ctx, client, entry, overwrite)
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", entry.Key, err)
			}
			continue
		}
		if written {
			imported++
		} else {
			skipped++
		}
	}

	message := fmt.Sprintf("已导入 %d 个，跳过 %d 个，失败 %d 个", imported, skipped, failed)
	if firstErr != nil {
		message += "（首个错误: " + firstErr.Error() + "）"
	}
	return config.BatchKeyResult{Success: failed == 0, Affected: imported, Failed: failed, Message: message}, nil
}

// CopyKeysToDatabase 把 key 复制到同一实例的另一个 DB（Redis 6.2+）
func (a *App) CopyKeysToDatabase(connID string, keys []string, dstDB int, overwrite bool) config.BatchKeyResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.BatchKeyResult{Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.BatchKeyResult{Message: err.Error()}
	}
	if len(keys) == 0 {
		return config.BatchKeyResult{Message: "请先选择要复制的 key"}
	}
	if len(keys) > redisbackend.ExportMaxKeys {
		return config.BatchKeyResult{Message: fmt.Sprintf("单次最多复制 %d 个 key", redisbackend.ExportMaxKeys)}
	}
	copied, skipped, failed, err := redisbackend.CopyKeysToDB(a.ctx, client, keys, dstDB, overwrite)
	message := fmt.Sprintf("DB %d：已复制 %d 个，跳过 %d 个，失败 %d 个", dstDB, copied, skipped, failed)
	if err != nil {
		message += "（" + err.Error() + "）"
	}
	return config.BatchKeyResult{Success: err == nil && failed == 0, Affected: copied, Failed: failed, Message: message}
}

// CopyKeysToConnection 把 key 复制到另一个已连接的实例，保留 TTL
func (a *App) CopyKeysToConnection(srcConnID, dstConnID string, keys []string, overwrite bool) (config.BatchKeyResult, error) {
	if srcConnID == dstConnID {
		return config.BatchKeyResult{Message: "源连接和目标连接相同"}, nil
	}
	if len(keys) == 0 {
		return config.BatchKeyResult{Message: "请先选择要复制的 key"}, nil
	}
	if len(keys) > redisbackend.ExportMaxKeys {
		return config.BatchKeyResult{Message: fmt.Sprintf("单次最多复制 %d 个 key", redisbackend.ExportMaxKeys)}, nil
	}
	src, err := a.manager.GetClient(srcConnID)
	if err != nil {
		return config.BatchKeyResult{Message: "源连接不可用: " + err.Error()}, nil
	}
	dst, err := a.manager.GetClient(dstConnID)
	if err != nil {
		return config.BatchKeyResult{Message: "目标连接未连接，请先连上再复制"}, nil
	}

	copied, skipped, failed := 0, 0, 0
	var firstErr error
	for _, key := range keys {
		entry, err := redisbackend.ReadKeyForExport(a.ctx, src, key)
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", key, err)
			}
			continue
		}
		written, err := redisbackend.WriteExportedKey(a.ctx, dst, entry, overwrite)
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", key, err)
			}
			continue
		}
		if written {
			copied++
		} else {
			skipped++
		}
	}

	message := fmt.Sprintf("已复制 %d 个，跳过 %d 个，失败 %d 个", copied, skipped, failed)
	if firstErr != nil {
		message += "（首个错误: " + firstErr.Error() + "）"
	}
	return config.BatchKeyResult{Success: failed == 0, Affected: copied, Failed: failed, Message: message}, nil
}

// GetSlowLog 读取当前连接的慢查询记录
func (a *App) GetSlowLog(connID string, count int) ([]config.SlowLogEntry, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return nil, err
	}
	return redisbackend.GetSlowLog(a.ctx, client, int64(count))
}

// ScanMemoryReport 对流式扫描 key 并生成内存体检报告
func (a *App) ScanMemoryReport(connID string, pattern string, maxKeys int64) (config.MemoryReport, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.MemoryReport{}, err
	}
	cfg, db, err := a.manager.GetConnectionState(connID)
	if err != nil {
		return config.MemoryReport{}, err
	}
	if cfg.IsCluster && !cfg.AllowClusterScan {
		normalized := strings.TrimSpace(pattern)
		if normalized == "" || normalized == "*" || strings.ContainsAny(normalized, "*?[") {
			return config.MemoryReport{}, errors.New("该集群连接未开启通配扫描，请填写精确或前缀明确的 pattern")
		}
	}

	a.memoryScanMu.Lock()
	if a.memoryScanCancel != nil {
		a.memoryScanMu.Unlock()
		return config.MemoryReport{}, errors.New("已有体检任务在执行，请先取消")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.memoryScanCancel = cancel
	a.memoryScanMu.Unlock()
	defer func() {
		cancel()
		a.memoryScanMu.Lock()
		a.memoryScanCancel = nil
		a.memoryScanMu.Unlock()
	}()

	lastEmit := time.Time{}
	onProgress := func(scanned, total int64) {
		if a.ctx == nil || time.Since(lastEmit) < 200*time.Millisecond {
			return
		}
		lastEmit = time.Now()
		percent := 0.0
		if total > 0 {
			percent = float64(scanned) / float64(total) * 100
			if percent > 100 {
				percent = 100
			}
		}
		wailsruntime.EventsEmit(a.ctx, "memory:progress", MemoryScanProgress{
			Scanned: scanned,
			Total:   total,
			Percent: percent,
		})
	}

	report, err := redisbackend.ScanMemory(ctx, client, pattern, maxKeys, onProgress)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return config.MemoryReport{}, errors.New("体检已取消")
		}
		return config.MemoryReport{}, err
	}
	report.ConnID = connID
	report.DB = db
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "memory:progress", MemoryScanProgress{
			Scanned: report.ScannedKeys,
			Total:   report.TotalKeys,
			Percent: 100,
		})
	}
	return report, nil
}

// CancelMemoryScan 取消正在执行的体检任务
func (a *App) CancelMemoryScan() config.OperationResult {
	a.memoryScanMu.Lock()
	cancel := a.memoryScanCancel
	a.memoryScanMu.Unlock()
	if cancel == nil {
		return config.OperationResult{Success: false, Message: "当前没有正在执行的体检"}
	}
	cancel()
	return config.OperationResult{Success: true}
}

func (a *App) ExecuteRedisCommand(connID, command string) (config.RedisConsoleResult, error) {
	if cfg, _, err := a.manager.GetConnectionState(connID); err == nil && cfg.ReadOnly {
		if !redisbackend.IsReadOnlyCommandAllowed(command) {
			// 不返回错误，方便前端识别为"需要确认"而不是执行失败
			return config.RedisConsoleResult{
				Success: false,
				Command: command,
				Blocked: true,
				Error:   "当前连接处于保护模式，该命令可能导致写入，需要确认后才能执行",
			}, nil
		}
	}
	return a.executeRedisCommand(connID, command)
}

// ExecuteRedisCommandForce 保护模式下的二次确认入口，前端确认后调用
func (a *App) ExecuteRedisCommandForce(connID, command string) (config.RedisConsoleResult, error) {
	return a.executeRedisCommand(connID, command)
}

func (a *App) executeRedisCommand(connID, command string) (config.RedisConsoleResult, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.RedisConsoleResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	return redisbackend.ExecuteRedisCommandWithState(ctx, client, command, connID, a.manager), nil
}

// ============================================================
// Value CRUD
// ============================================================

// GetValue 读取 Value（支持 cursor/offset/stream ID 分页）。第一页使用空 streamStart。
func (a *App) GetValue(connID, key string, cursor uint64, offset int, zsetSort, streamStart, keyType string) (config.KeyValue, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.KeyValue{}, err
	}
	settings, err := config.GetSettings()
	if err != nil {
		return config.KeyValue{}, err
	}
	// 每次读取用独立超时 context，防止大 key 阻塞整个应用
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	_, db, _ := a.manager.GetConnectionState(connID)
	scope := connID + "|" + strconv.Itoa(db)
	return redisbackend.GetValue(ctx, client, key, settings, cursor, offset, zsetSort, streamStart, scope, keyType)
}

// SearchValue 按 pattern 搜索 key 内成员（Hash/Set 支持 * 通配符，ZSet 使用 Redis glob，List 使用子串匹配）
// exact=true 时，Set 使用 SIsMember，Hash 使用 HGet 进行精确匹配。
// cursor 在 Hash/Set 模糊搜索时用于继续拉取下一页结果。
func (a *App) SearchValue(connID, key, keyType, pattern string, exact bool, cursor uint64) (config.KeyValue, error) {
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.KeyValue{}, err
	}
	settings, err := config.GetSettings()
	if err != nil {
		return config.KeyValue{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	return redisbackend.SearchValue(ctx, client, key, keyType, pattern, settings, exact, cursor)
}

// SetString 设置 string
func (a *App) SetString(connID, key, value string) config.OperationResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.SetStringPreserveTTL(a.ctx, client, key, value); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// CreateKey 创建一个新 key。
func (a *App) CreateKey(connID string, req config.CreateKeyRequest) config.OperationResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	if err := redisbackend.CreateKey(ctx, client, req); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// HSet 设置 hash field
func (a *App) HSet(connID, key, field, value string) config.OperationResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
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
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
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
	defer a.invalidateListCache(connID, key)
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
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
	defer a.invalidateListCache(connID, key)
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
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
	defer a.invalidateListCache(connID, key)
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
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
	defer a.invalidateListCache(connID, key)
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
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
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
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
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.SRem(a.ctx, client, key, member); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// RenameSetMember 原子重命名 set 成员
func (a *App) RenameSetMember(connID, key, source, destination string) config.OperationResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	if err := redisbackend.RenameSetMember(ctx, client, key, source, destination); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// ZAdd 向 zset 添加成员
func (a *App) ZAdd(connID, key, member string, score float64) config.OperationResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
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
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	if err := redisbackend.ZRem(a.ctx, client, key, member); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// RenameZSetMember 原子重命名 zset 成员并保留当前 score
func (a *App) RenameZSetMember(connID, key, source, destination string) config.OperationResult {
	if err := a.ensureWritable(connID); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	if err := redisbackend.RenameZSetMember(ctx, client, key, source, destination); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// ============================================================
// 设置管理
// ============================================================

// GetSettings 获取全局设置
func (a *App) GetSettings() (config.AppSettings, error) {
	return config.GetSettings()
}

// SaveSettings 保存全局设置
// GetCommandHistory 命令台历史（持久化，最多 100 条）
func (a *App) GetCommandHistory() []string {
	return config.GetCommandHistory()
}

// PushCommandHistory 记录一条执行过的命令
func (a *App) PushCommandHistory(command string) config.OperationResult {
	if err := config.AppendCommandHistory(command); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// ClearCommandHistory 清空命令历史
func (a *App) ClearCommandHistory() config.OperationResult {
	if err := config.ClearCommandHistory(); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// CompleteOnboarding 记录首次使用向导已看过
func (a *App) CompleteOnboarding() config.OperationResult {
	if err := config.MarkOnboardingDone(); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// ExportConnectionsToFile 导出全部连接配置（含明文密码，前端会二次确认）
func (a *App) ExportConnectionsToFile() (string, error) {
	payload, err := config.ExportConnections()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出连接配置",
		DefaultFilename: "literedis-connections-" + stamp + ".json",
		Filters:         []wailsruntime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}

// ImportConnectionsFromFile 导入连接配置：同 ID 覆盖、同组重名自动加后缀
func (a *App) ImportConnectionsFromFile() (config.ImportSummary, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入连接配置",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "JSON", Pattern: "*.json"},
		},
	})
	if err != nil {
		return config.ImportSummary{}, err
	}
	if path == "" {
		return config.ImportSummary{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config.ImportSummary{}, err
	}
	var payload config.ConfigExport
	if err := json.Unmarshal(data, &payload); err != nil {
		return config.ImportSummary{}, fmt.Errorf("不是有效的导出文件: %w", err)
	}
	if payload.App != "" && payload.App != "LiteRedis" {
		return config.ImportSummary{}, fmt.Errorf("不是 LiteRedis 的配置文件")
	}
	summary, err := config.ImportConnections(payload.Connections)
	if err != nil {
		return summary, err
	}
	// 被覆盖的连接如果传输参数变了，让运行中的实例按新配置重连
	for _, id := range summary.UpdatedIDs {
		cfg, cfgErr := config.GetConnection(id)
		if cfgErr != nil || cfg == nil {
			continue
		}
		if a.manager.ApplyConnectionConfig(*cfg) {
			a.registry.UnpublishConn(id)
		}
	}
	return summary, nil
}

// MarkVersionSeen 记录已经看过某个版本的更新说明，保证每个版本只提示一次
func (a *App) MarkVersionSeen(version string) config.OperationResult {
	if err := config.MarkVersionSeen(version); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

func (a *App) SaveSettings(s config.AppSettings) config.OperationResult {
	if err := config.SaveSettings(s); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	// 本地 API 开关/端口/调试开关变化时同步配置并重启服务
	a.configureLocalAPI(s)
	if err := a.apiServer.Apply(s); err != nil {
		config.AppendDebugLog("[localapi] apply settings failed: %v", err)
		return config.OperationResult{Success: false, Message: "设置已保存，但本地 API 服务启动失败: " + err.Error()}
	}
	return config.OperationResult{Success: true}
}

// ============================================================
// 本地数据提取 API
// ============================================================

// PublishKeyValue 前端上报提取数据（点击「浏览器解析」时写入唯一槽位）
func (a *App) PublishKeyValue(connID string, kv config.KeyValue, db int, nonce string) config.OperationResult {
	// 记录连接名（展示用）
	connName := ""
	if cfg, _, err := a.manager.GetConnectionState(connID); err == nil {
		connName = cfg.Name
	}
	if err := a.registry.Publish(connID, kv.Key, kv, db, connName, nonce); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}

// LocalAPIInfo 本地 API 运行信息
type LocalAPIInfo struct {
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
	Error   string `json:"error,omitempty"`
}

// GetLocalAPIInfo 获取本地 API 服务状态与 token
func (a *App) GetLocalAPIInfo() LocalAPIInfo {
	info := a.apiServer.Info()
	return LocalAPIInfo{Running: info.Running, Port: info.Port, Token: info.Token}
}

// RegenerateLocalAPIToken 换发本地 API token（旧 token 立即失效）
func (a *App) RegenerateLocalAPIToken() config.OperationResult {
	if err := a.apiServer.RegenerateToken(); err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}
	return config.OperationResult{Success: true}
}
