package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	debugLogPath string
	debugLogMu   sync.Mutex
)

func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	appDir := filepath.Join(dir, "LiteRedis")
	_ = os.MkdirAll(appDir, 0755)
	debugLogPath = filepath.Join(appDir, "connection-debug.log")
}

func DebugLogPath() string {
	return debugLogPath
}

func AppendDebugLog(format string, args ...any) {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()

	file, err := os.OpenFile(debugLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	line := fmt.Sprintf(format, args...)
	_, _ = file.WriteString(fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), line))
}
