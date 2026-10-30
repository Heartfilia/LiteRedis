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

func TestMarkVersionSeenAndPreserveOnSave(t *testing.T) {
	useTestConfigPath(t)

	if err := MarkVersionSeen("v1.2.3"); err != nil {
		t.Fatalf("MarkVersionSeen: %v", err)
	}
	settings, err := GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if settings.LastSeenVersion != "v1.2.3" {
		t.Fatalf("expected last seen version v1.2.3, got %q", settings.LastSeenVersion)
	}

	// 前端保存设置时不会带这个字段，保存后必须保留原值
	settings.LastSeenVersion = ""
	settings.ProxyURL = "http://127.0.0.1:7890"
	if err := SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	after, err := GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after save: %v", err)
	}
	if after.LastSeenVersion != "v1.2.3" {
		t.Fatalf("last seen version was cleared by SaveSettings: %q", after.LastSeenVersion)
	}
	if after.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("unrelated setting not saved: %q", after.ProxyURL)
	}
}

func TestMarkVersionSeenIgnoresBlank(t *testing.T) {
	useTestConfigPath(t)
	if err := MarkVersionSeen("   "); err != nil {
		t.Fatalf("MarkVersionSeen(blank): %v", err)
	}
	settings, err := GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if settings.LastSeenVersion != "" {
		t.Fatalf("blank version should not be stored, got %q", settings.LastSeenVersion)
	}
}

func TestSaveConnectionRejectsDuplicateNameInSameGroup(t *testing.T) {
	useTestConfigPath(t)

	first, err := SaveConnection(ConnectionConfig{Name: "dev", Group: "A", Host: "127.0.0.1", Port: 6379})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}

	// 同组同名（大小写、首尾空格不同）应被拒绝
	if _, err := SaveConnection(ConnectionConfig{Name: "  DEV ", Group: " A ", Host: "127.0.0.1", Port: 6379}); err == nil {
		t.Fatal("expected duplicate name in same group to be rejected")
	} else if !strings.Contains(err.Error(), "已存在同名连接") {
		t.Fatalf("unexpected error: %v", err)
	}

	// 不同分组可以同名
	if _, err := SaveConnection(ConnectionConfig{Name: "dev", Group: "B", Host: "127.0.0.1", Port: 6379}); err != nil {
		t.Fatalf("same name in another group should be allowed: %v", err)
	}

	// 更新自己（名字不变）不应被自己挡住
	first.Host = "10.0.0.1"
	if _, err := SaveConnection(first); err != nil {
		t.Fatalf("update self: %v", err)
	}

	// 挪到 B 组会和已有的 dev 撞名
	first.Group = "B"
	if _, err := SaveConnection(first); err == nil {
		t.Fatal("expected moving into a group with the same name to be rejected")
	}

	// 空名称直接拒绝
	if _, err := SaveConnection(ConnectionConfig{Name: "   ", Group: "C", Host: "127.0.0.1", Port: 6379}); err == nil {
		t.Fatal("expected blank name to be rejected")
	}
}

func TestReorderRejectsDuplicateNameInGroup(t *testing.T) {
	useTestConfigPath(t)

	a, err := SaveConnection(ConnectionConfig{Name: "dev", Group: "A", Host: "127.0.0.1", Port: 6379})
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	b, err := SaveConnection(ConnectionConfig{Name: "dev", Group: "B", Host: "127.0.0.1", Port: 6379})
	if err != nil {
		t.Fatalf("create b: %v", err)
	}

	err = ReorderConnections([]ConnectionOrderItem{
		{ID: a.ID, Group: "A", SortOrder: 0},
		{ID: b.ID, Group: "A", SortOrder: 1},
	})
	if err == nil {
		t.Fatal("expected moving into a group with the same name to be rejected")
	}

	// 拒绝后不能落盘：b 仍应在 B 组
	list, err := ListConnections()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, conn := range list {
		if conn.ID == b.ID && conn.Group != "B" {
			t.Fatalf("rejected reorder should not be persisted, group = %q", conn.Group)
		}
	}

	// 同组内排序（不改分组）不应被挡
	if err := ReorderConnections([]ConnectionOrderItem{
		{ID: a.ID, Group: "A", SortOrder: 0},
		{ID: b.ID, Group: "B", SortOrder: 1},
	}); err != nil {
		t.Fatalf("plain reorder should be allowed: %v", err)
	}
}

func TestLegacyDuplicateNamesAreNormalized(t *testing.T) {
	path := useTestConfigPath(t)
	payload := `{"version":1,"connections":[
		{"id":"a","name":"dev","group":"A","host":"h","port":6379,"sort_order":0},
		{"id":"b","name":" DEV ","group":" A ","host":"h","port":6379,"sort_order":1},
		{"id":"c","name":"dev","group":"B","host":"h","port":6379,"sort_order":2}
	],"settings":{}}`
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}

	conns, err := ListConnections()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[string]string{}
	for _, conn := range conns {
		got[conn.ID] = conn.Name
	}
	if got["a"] != "dev" {
		t.Fatalf("a should keep its name, got %q", got["a"])
	}
	if got["b"] != "DEV (2)" {
		t.Fatalf("second duplicate should be renamed, got %q", got["b"])
	}
	if got["c"] != "dev" {
		t.Fatalf("same name in another group must stay untouched, got %q", got["c"])
	}

	// 迁移后保存老连接不应该被同名挡住（自身被排除）
	first := conns[0]
	first.Host = "10.0.0.2"
	if _, err := SaveConnection(first); err != nil {
		t.Fatalf("saving migrated connection: %v", err)
	}

	// 保存后改名结果落盘
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "DEV (2)") {
		t.Fatalf("expected renamed connection to be persisted, file: %s", data)
	}
}

func TestMarkOnboardingDoneAndPreserveOnSave(t *testing.T) {
	useTestConfigPath(t)

	settings, err := GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if settings.OnboardingDone {
		t.Fatal("onboarding should default to false")
	}

	if err := MarkOnboardingDone(); err != nil {
		t.Fatalf("MarkOnboardingDone: %v", err)
	}
	settings, _ = GetSettings()
	if !settings.OnboardingDone {
		t.Fatal("onboarding flag was not persisted")
	}

	// 前端保存设置时不会带这个字段，保存后必须保留
	settings.OnboardingDone = false
	settings.ProxyURL = "http://127.0.0.1:7890"
	if err := SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	after, _ := GetSettings()
	if !after.OnboardingDone {
		t.Fatal("onboarding flag was cleared by SaveSettings")
	}
}

func TestImportConnectionsMergesRenamesAndSkips(t *testing.T) {
	useTestConfigPath(t)

	existing, err := SaveConnection(ConnectionConfig{Name: "dev", Group: "A", Host: "h1", Port: 6379})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	summary, err := ImportConnections([]ConnectionConfig{
		{ID: existing.ID, Name: "dev", Group: "A", Host: "h2", Port: 6379},
		{Name: "dev", Group: "A", Host: "h3", Port: 6379},
		{Name: "  ", Group: "A", Host: "h4", Port: 6379},
		{Name: "other", Group: "B", Host: "h5", Port: 6379},
	})
	if err != nil {
		t.Fatalf("ImportConnections: %v", err)
	}
	if summary.Updated != 1 || summary.Added != 2 || summary.Skipped != 1 || summary.Renamed != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.UpdatedIDs) != 1 || summary.UpdatedIDs[0] != existing.ID {
		t.Fatalf("updated ids not reported: %+v", summary.UpdatedIDs)
	}

	list, err := ListConnections()
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(list))
	}
	byName := map[string]ConnectionConfig{}
	for _, c := range list {
		byName[c.Name] = c
	}
	if byName["dev"].Host != "h2" {
		t.Fatalf("same id should be overwritten, got host %q", byName["dev"].Host)
	}
	if _, ok := byName["dev (2)"]; !ok {
		t.Fatalf("duplicate name should be renamed, got %+v", byName)
	}
	if _, ok := byName["other"]; !ok {
		t.Fatalf("new connection missing: %+v", byName)
	}
}

func TestExportConnectionsSortedAndComplete(t *testing.T) {
	useTestConfigPath(t)

	if _, err := SaveConnection(ConnectionConfig{Name: "b", Group: "A", Host: "h", Port: 6379}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveConnection(ConnectionConfig{Name: "a", Group: "A", Host: "h", Port: 6379}); err != nil {
		t.Fatal(err)
	}

	payload, err := ExportConnections()
	if err != nil {
		t.Fatalf("ExportConnections: %v", err)
	}
	if payload.App != "LiteRedis" || len(payload.Connections) != 2 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Connections[0].Name != "b" || payload.Connections[1].Name != "a" {
		t.Fatalf("export should follow sort order, got %q then %q", payload.Connections[0].Name, payload.Connections[1].Name)
	}
}
