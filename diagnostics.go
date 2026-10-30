package main

import (
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"LiteRedis/backend/config"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DiagnosticConnection 诊断页里的单条连接状态
type DiagnosticConnection struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Connected bool   `json:"connected"`
	LatencyMS int64  `json:"latency_ms"` // -1 表示未连接 / 未测
	Error     string `json:"error,omitempty"`
}

// Diagnostics 诊断页需要的全部信息
type Diagnostics struct {
	AppVersion      string                 `json:"app_version"`
	Platform        string                 `json:"platform"`
	ConfigPath      string                 `json:"config_path"`
	DebugLogPath    string                 `json:"debug_log_path"`
	DebugLogSize    int64                  `json:"debug_log_size"`
	LocalAPIEnabled bool                   `json:"local_api_enabled"`
	LocalAPIRunning bool                   `json:"local_api_running"`
	LocalAPIPort    int                    `json:"local_api_port"`
	Connections     []DiagnosticConnection `json:"connections"`
	LogTail         []string               `json:"log_tail"`
}

// GetDiagnostics 汇总版本、路径、连接延迟和最近日志，供设置页的诊断面板展示
func (a *App) GetDiagnostics() Diagnostics {
	info := a.apiServer.Info()
	settings, _ := config.GetSettings()
	conns, _ := a.ListConnections()

	out := Diagnostics{
		AppVersion:      AppVersion,
		Platform:        runtime.GOOS + "/" + runtime.GOARCH,
		ConfigPath:      config.ConfigPath(),
		DebugLogPath:    config.DebugLogPath(),
		LocalAPIEnabled: settings.LocalAPIEnabled,
		LocalAPIRunning: info.Running,
		LocalAPIPort:    info.Port,
		Connections:     make([]DiagnosticConnection, 0, len(conns)),
	}
	if stat, err := os.Stat(out.DebugLogPath); err == nil {
		out.DebugLogSize = stat.Size()
	}
	out.LogTail, _ = tailFile(out.DebugLogPath, 200)

	// 并行测延迟，单条最长 2s，避免诊断页被某个不可达实例拖住
	var wg sync.WaitGroup
	items := make([]DiagnosticConnection, len(conns))
	for i, c := range conns {
		items[i] = DiagnosticConnection{
			ID:        c.ID,
			Name:      c.Name,
			Host:      c.Host,
			Port:      c.Port,
			Connected: a.IsConnected(c.ID),
			LatencyMS: -1,
		}
		if !items[i].Connected {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			done := make(chan error, 1)
			start := time.Now()
			go func() { done <- a.manager.Ping(conns[idx].ID) }()
			select {
			case err := <-done:
				if err != nil {
					items[idx].Error = err.Error()
					return
				}
				items[idx].LatencyMS = time.Since(start).Milliseconds()
			case <-time.After(2 * time.Second):
				items[idx].Error = "ping 超时（>2s）"
			}
		}(i)
	}
	wg.Wait()
	out.Connections = items
	return out
}

// ExportDebugLog 把日志另存为文件，方便用户发给你排查
func (a *App) ExportDebugLog() (string, error) {
	src := config.DebugLogPath()
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("（日志为空）\n")
		} else {
			return "", err
		}
	}
	stamp := time.Now().Format("20060102-150405")
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出诊断日志",
		DefaultFilename: "literedis-diagnostics-" + stamp + ".log",
		Filters:         []wailsruntime.FileFilter{{DisplayName: "Log", Pattern: "*.log"}},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

// tailFile 读取文件末尾最多 maxLines 行；文件过大时只读尾部 256KB
func tailFile(path string, maxLines int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	const tailBytes = 256 << 10
	offset := int64(0)
	if stat.Size() > tailBytes {
		offset = stat.Size() - tailBytes
	}
	buf := make([]byte, stat.Size()-offset)
	if _, err := file.ReadAt(buf, offset); err != nil && len(buf) > 0 {
		// 读到的部分仍然可用，忽略 io.EOF 之类的错误
	}
	text := strings.TrimRight(string(buf), "\n")
	if text == "" {
		return []string{}, nil
	}
	lines := strings.Split(text, "\n")
	if offset > 0 && len(lines) > 1 {
		lines = lines[1:] // 首行可能被截断
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}
