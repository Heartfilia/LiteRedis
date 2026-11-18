package config

import "testing"

func TestSaveConnectionNormalizesIconText(t *testing.T) {
	useTestConfigPath(t)

	saved, err := SaveConnection(ConnectionConfig{
		Name:     "icon",
		Host:     "127.0.0.1",
		Port:     6379,
		IconText: "  AB\nCDE  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.IconText != "ABCD" {
		t.Fatalf("saved icon text = %q, want ABCD", saved.IconText)
	}

	connections, err := ListConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0].IconText != "ABCD" {
		t.Fatalf("reloaded icon text = %+v", connections)
	}
}

func TestImportConfigurationNormalizesIconText(t *testing.T) {
	useTestConfigPath(t)

	summary, err := ImportConfiguration([]ConnectionConfig{{
		Name:     "imported",
		Host:     "10.0.0.9",
		Port:     6379,
		IconText: "缓存一号库",
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Added != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	connections, err := ListConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0].IconText != "缓存一号" {
		t.Fatalf("imported icon text = %+v", connections)
	}
}
