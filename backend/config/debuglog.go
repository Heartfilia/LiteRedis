package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	debugLogPath  string
	debugLogMu    sync.Mutex
	debugLogMaxSz int64 = 5 * 1024 * 1024 // 超过后轮转，变量便于测试
)

func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	appDir := filepath.Join(dir, "LiteRedis")
	_ = os.MkdirAll(appDir, 0700)
	debugLogPath = filepath.Join(appDir, "connection-debug.log")
}

// setDebugLogForTest 供测试注入路径与阈值
func setDebugLogForTest(path string, maxSize int64) {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()
	debugLogPath = path
	debugLogMaxSz = maxSize
}

func DebugLogPath() string {
	return debugLogPath
}

// rotateIfNeededLocked 在超过阈值时将当前日志重命名为 .old（覆盖旧 .old）。
func rotateIfNeededLocked() error {
	info, err := os.Stat(debugLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() < debugLogMaxSz {
		return nil
	}
	oldPath := debugLogPath + ".old"
	if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(debugLogPath, oldPath); err != nil {
		return err
	}
	return nil
}

func AppendDebugLog(format string, args ...any) {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()

	// 轮转失败不阻塞日志写入
	_ = rotateIfNeededLocked()

	file, err := os.OpenFile(debugLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer file.Close()

	line := fmt.Sprintf(format, args...)
	_, _ = file.WriteString(fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), line))
}
