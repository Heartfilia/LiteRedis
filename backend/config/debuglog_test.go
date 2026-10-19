package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func useTestDebugLog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "connection-debug.log")
	oldPath := debugLogPath
	oldMax := debugLogMaxSz
	debugLogPath = path
	debugLogMaxSz = 300 // 小阈值便于触发轮转（每行约 100 字节，可容纳 2-3 行）
	t.Cleanup(func() {
		debugLogPath = oldPath
		debugLogMaxSz = oldMax
	})
	return path
}

func TestAppendDebugLogRotatesWhenOversized(t *testing.T) {
	path := useTestDebugLog(t)

	for i := 0; i < 10; i++ {
		AppendDebugLog("rotate-test line %d with some padding to exceed the tiny threshold", i)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("current log missing after rotation: %v", err)
	}
	oldData, err := os.ReadFile(path + ".old")
	if err != nil {
		t.Fatalf("rotated log missing: %v", err)
	}
	if !strings.Contains(string(oldData), "rotate-test") {
		t.Fatalf("rotated log missing content: %q", oldData)
	}

	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(current), "rotate-test line 9") {
		t.Fatalf("current log missing latest line: %q", current)
	}
}

func TestAppendDebugLogRotatesOverwritesOldBackup(t *testing.T) {
	path := useTestDebugLog(t)
	if err := os.WriteFile(path+".old", []byte("stale backup"), 0600); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		AppendDebugLog("overwrite-test line %d with some padding to exceed the tiny threshold", i)
	}

	oldData, err := os.ReadFile(path + ".old")
	if err != nil {
		t.Fatalf("rotated log missing: %v", err)
	}
	if strings.Contains(string(oldData), "stale backup") {
		t.Fatal("stale .old backup was not overwritten")
	}
	if !strings.Contains(string(oldData), "overwrite-test") {
		t.Fatalf("rotated log missing new content: %q", oldData)
	}
}

func TestAppendDebugLogUsesRestrictedPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file ACLs are not represented by os.FileMode")
	}
	path := useTestDebugLog(t)

	AppendDebugLog("perm-test")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("debug log permissions = %o, want 600", got)
	}
}
