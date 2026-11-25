package config

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestNormalizeConnectionEnvironment(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  string
	}{
		{"", ""}, {" \t", ""},
		{"development", "development"}, {"staging", "staging"}, {"production", "production"},
		{" Production \n", "production"}, {"STAGING", "staging"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizeConnectionEnvironment(tt.input)
			if err != nil || got != tt.want {
				t.Fatalf("normalize(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
			}
		})
	}
	for _, input := range []string{"prod", "testing", "unknown", "production\x00", "production staging"} {
		if _, err := NormalizeConnectionEnvironment(input); err == nil {
			t.Errorf("normalize(%q) accepted an unknown environment", input)
		}
	}
}

func TestConnectionEnvironmentSaveUpdateCopyAndReadOnly(t *testing.T) {
	useTestConfigPath(t)
	for _, environment := range []string{"", "development", "staging", "production"} {
		for _, readOnly := range []bool{false, true} {
			name := environment + "-writable"
			if readOnly {
				name = environment + "-protected"
			}
			saved, err := SaveConnection(ConnectionConfig{Name: name, Host: "localhost", Environment: environment, ReadOnly: readOnly})
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := GetConnection(saved.ID)
			if err != nil || loaded == nil {
				t.Fatalf("load saved connection: %v, %v", loaded, err)
			}
			if loaded.Environment != environment || loaded.ReadOnly != readOnly {
				t.Fatalf("environment or protection changed: %+v", loaded)
			}
			// The existing copy flow copies the struct and clears the ID.
			copied := *loaded
			copied.ID, copied.Name = "", name+"-copy"
			copySaved, err := SaveConnection(copied)
			if err != nil || copySaved.Environment != environment || copySaved.ReadOnly != readOnly || copySaved.ID == saved.ID {
				t.Fatalf("copy failed to preserve environment/protection: %+v, %v", copySaved, err)
			}
			saved.Environment = " STAGING "
			updated, err := SaveConnection(saved)
			if err != nil || updated.Environment != "staging" || updated.ReadOnly != readOnly {
				t.Fatalf("update failed: %+v, %v", updated, err)
			}
		}
	}
}

func TestConnectionEnvironmentRejectsSaveAndImportWithoutWriting(t *testing.T) {
	path := useTestConfigPath(t)
	existing, err := SaveConnection(ConnectionConfig{Name: "existing", Host: "localhost", Environment: "production"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertUnchanged := func() {
		t.Helper()
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("rejected operation modified disk: %v", err)
		}
	}
	for _, id := range []string{"", existing.ID} {
		if _, err := SaveConnection(ConnectionConfig{ID: id, Name: "invalid", Host: "localhost", Environment: "prod"}); err == nil {
			t.Fatal("save accepted unknown environment")
		}
		assertUnchanged()
	}
	incoming := []ConnectionConfig{
		{ID: existing.ID, Name: "updated", Host: "localhost", Environment: " STAGING "},
		{Name: "new", Host: "localhost", Environment: "development"},
		{Name: "invalid", Host: "localhost", Environment: "prod"},
	}
	if _, err := ImportConnections(incoming); err == nil {
		t.Fatal("import accepted unknown environment")
	}
	assertUnchanged()
	if incoming[0].Environment != " STAGING " {
		t.Fatal("import modified caller's input")
	}
	if err := saveStore(&ConfigStore{Connections: []ConnectionConfig{{Environment: "prod"}}}); err == nil {
		t.Fatal("saveStore accepted unknown environment")
	}
	assertUnchanged()
}

func TestConnectionEnvironmentLegacyAndExportImportRoundTrip(t *testing.T) {
	path := useTestConfigPath(t)
	legacy := []byte(`{"version":1,"connections":[{"id":"legacy","name":"legacy","host":"localhost","read_only":true}],"settings":{}}`)
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	old, err := GetConnection("legacy")
	if err != nil || old == nil || old.Environment != "" || !old.ReadOnly {
		t.Fatalf("old config is not compatible: %+v, %v", old, err)
	}
	if _, err := SaveConnection(*old); err != nil {
		t.Fatal(err)
	}
	for _, environment := range []string{"development", "staging", "production"} {
		if _, err := SaveConnection(ConnectionConfig{Name: environment, Host: "localhost", Environment: environment}); err != nil {
			t.Fatal(err)
		}
	}
	exported, err := ExportConnections()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ConfigExport
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	legacyJSON, err := json.Marshal(decoded.Connections[0])
	if err != nil || bytes.Contains(legacyJSON, []byte(`"environment"`)) {
		t.Fatalf("empty environment must be omitted: %s, %v", legacyJSON, err)
	}
	// Import into another isolated store, never the user's real configuration.
	useTestConfigPath(t)
	summary, err := ImportConnections(decoded.Connections)
	if err != nil || summary.Added != 4 {
		t.Fatalf("round-trip import: %+v, %v", summary, err)
	}
	got, err := ListConnections()
	if err != nil || len(got) != 4 {
		t.Fatalf("round-trip list: %+v, %v", got, err)
	}
	for i, conn := range got {
		want := decoded.Connections[i]
		if conn.Environment != want.Environment || conn.ReadOnly != want.ReadOnly {
			t.Errorf("round-trip changed environment/protection: %+v, want %+v", conn, want)
		}
	}
	// Same-ID imports update and normalize the label without changing protection.
	got[0].Environment = " PRODUCTION "
	if _, err := ImportConnections(got[:1]); err != nil {
		t.Fatal(err)
	}
	updated, err := GetConnection(got[0].ID)
	if err != nil || updated == nil || updated.Environment != "production" || !updated.ReadOnly {
		t.Fatalf("same-ID import failed: %+v, %v", updated, err)
	}
}
