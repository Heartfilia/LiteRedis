package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// WindowState 窗口位置与尺寸
type WindowState struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

var (
	winMu      sync.Mutex
	windowPath string
)

func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	appDir := filepath.Join(dir, "LiteRedis")
	_ = os.MkdirAll(appDir, 0755)
	windowPath = filepath.Join(appDir, "window.json")
}

// LoadWindowState 从磁盘读取窗口状态，失败时返回默认值
func LoadWindowState() WindowState {
	winMu.Lock()
	defer winMu.Unlock()
	data, err := os.ReadFile(windowPath)
	if err != nil {
		return WindowState{X: -1, Y: -1, Width: 1280, Height: 800}
	}
	var ws WindowState
	if err := json.Unmarshal(data, &ws); err != nil {
		return WindowState{X: -1, Y: -1, Width: 1280, Height: 800}
	}
	// 合法性保护
	if ws.Width < 800 {
		ws.Width = 1280
	}
	if ws.Height < 500 {
		ws.Height = 800
	}
	return ws
}

// SaveWindowState 将窗口状态写入磁盘
func SaveWindowState(ws WindowState) error {
	winMu.Lock()
	defer winMu.Unlock()
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(windowPath, data, 0644)
}
