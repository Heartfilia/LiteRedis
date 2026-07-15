package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func useTestConfigPath(t *testing.T) string {
	t.Helper()
	oldPath := configPath
	configPath = filepath.Join(t.TempDir(), "connections.json")
	t.Cleanup(func() {
		configPath = oldPath
	})
	return configPath
}

func TestLoadStorePreservesAndBacksUpCorruptJSON(t *testing.T) {
	path := useTestConfigPath(t)
	corrupt := []byte(`{"version":1,"connections":[`)
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := loadStore(); err == nil || !strings.Contains(err.Error(), "invalid config JSON") {
		t.Fatalf("expected explicit corrupt config error, got %v", err)
	}
	if _, err := loadStore(); err == nil {
		t.Fatal("second corrupt config read unexpectedly succeeded")
	}

	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != string(corrupt) {
		t.Fatalf("original config was modified: %q", original)
	}
	backups, err := filepath.Glob(path + ".corrupt-*.bak")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("backup count = %d, want 1: %v", len(backups), backups)
	}
	backup, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(corrupt) {
		t.Fatalf("backup content = %q, want %q", backup, corrupt)
	}
}

func TestSaveConnectionDoesNotOverwriteCorruptConfig(t *testing.T) {
	path := useTestConfigPath(t)
	corrupt := []byte(`not-json`)
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := SaveConnection(ConnectionConfig{Name: "must-not-save"}); err == nil {
		t.Fatal("SaveConnection accepted a corrupt config store")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != string(corrupt) {
		t.Fatalf("corrupt config was overwritten: %q", original)
	}
}

func TestConfigFilePermissionsAreRestricted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file ACLs are not represented by os.FileMode")
	}
	path := useTestConfigPath(t)
	store := &ConfigStore{Version: 1, Connections: []ConnectionConfig{}, Settings: DefaultSettings()}
	if err := saveStore(store); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadStore(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
}

func TestSaveSettingsUsesDocumentedLoadDefaults(t *testing.T) {
	useTestConfigPath(t)
	if err := saveStore(&ConfigStore{Version: 1, Connections: []ConnectionConfig{}, Settings: DefaultSettings()}); err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(AppSettings{}); err != nil {
		t.Fatal(err)
	}
	got, err := GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultSettings()
	if got.KeyScanCount != want.KeyScanCount ||
		got.HashLoadCount != want.HashLoadCount ||
		got.ListLoadCount != want.ListLoadCount ||
		got.SetLoadCount != want.SetLoadCount ||
		got.ZSetLoadCount != want.ZSetLoadCount ||
		got.StreamLoadCount != want.StreamLoadCount {
		t.Fatalf("load defaults = %+v, want %+v", got, want)
	}
}
