package config

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func testSSHConfig() SSHConfig {
	return SSHConfig{Host: "jump.example.com", Port: 22, User: "ubuntu", Password: "  secret  ", Passphrase: "  phrase  "}
}

func TestSSHProfileMigrationIsStableAndPreservesCredentials(t *testing.T) {
	path := useTestConfigPath(t)
	ssh := testSSHConfig()
	other := ssh
	other.Password = "different"
	store := ConfigStore{Version: 1, Settings: DefaultSettings(), Connections: []ConnectionConfig{
		{ID: "one", Name: "one", SSHEnabled: true, SSH: &ssh},
		{ID: "two", Name: "two", SSHEnabled: true, SSH: &ssh},
		{ID: "three", Name: "three", SSH: &other},
	}}
	data, _ := json.Marshal(store)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	first, err := ListSSHProfiles()
	if err != nil || len(first) != 2 {
		t.Fatalf("profiles=%+v err=%v", first, err)
	}
	second, _ := ListSSHProfiles()
	if !reflect.DeepEqual(first, second) {
		t.Fatal("migration IDs changed on read")
	}
	unchanged, _ := os.ReadFile(path)
	if string(unchanged) != string(data) {
		t.Fatal("read unexpectedly rewrote user config")
	}
	connections, _ := ListConnections()
	byID := map[string]ConnectionConfig{}
	for _, conn := range connections {
		byID[conn.ID] = conn
	}
	if byID["one"].SSHProfileID == "" || byID["one"].SSHProfileID != byID["two"].SSHProfileID || byID["three"].SSHProfileID == byID["one"].SSHProfileID {
		t.Fatal("incorrect legacy associations")
	}
	if first[0].SSH.Password != ssh.Password || first[0].SSH.Passphrase != ssh.Passphrase {
		t.Fatal("credential whitespace lost")
	}
	if err := SaveSettings(DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	third, _ := ListSSHProfiles()
	if !reflect.DeepEqual(first, third) {
		t.Fatal("migration changed after persistence")
	}
}

func TestSSHProfileCRUDReferencesAndPortableExport(t *testing.T) {
	useTestConfigPath(t)
	profile, err := SaveSSHProfile(SSHProfile{Name: " Jump ", SSH: testSSHConfig()})
	if err != nil || profile.ID == "" || profile.Name != "Jump" {
		t.Fatalf("save=%+v %v", profile, err)
	}
	if _, err := SaveSSHProfile(SSHProfile{Name: "jump", SSH: testSSHConfig()}); err == nil {
		t.Fatal("accepted duplicate name")
	}
	conn, err := SaveConnection(ConnectionConfig{Name: "redis", Host: "localhost", SSHEnabled: true, SSHProfileID: profile.ID})
	if err != nil || conn.SSH == nil || conn.SSH.Password != profile.SSH.Password {
		t.Fatalf("resolution=%+v %v", conn, err)
	}
	if err := DeleteSSHProfile(profile.ID); err == nil {
		t.Fatal("deleted referenced account")
	}
	profile.SSH.Password = " new password "
	profile, err = SaveSSHProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := GetConnection(conn.ID)
	if err != nil || resolved.SSH.Password != profile.SSH.Password {
		t.Fatalf("stale credentials: %+v %v", resolved, err)
	}
	// Ignore untrusted inline credentials when an existing account is selected.
	stale := testSSHConfig()
	resolved.SSH = &stale
	actual, err := ResolveConnectionSSH(*resolved)
	if err != nil || actual.SSH.Password != profile.SSH.Password {
		t.Fatal("inline snapshot overrode selected account")
	}
	exported, err := ExportConnections()
	if err != nil || exported.Connections[0].SSHProfileID != "" || exported.Connections[0].SSH.Password != profile.SSH.Password {
		t.Fatalf("nonportable export: %+v %v", exported, err)
	}
	if _, err := SaveConnection(ConnectionConfig{Name: "bad", SSHEnabled: true, SSHProfileID: "missing", SSH: &stale}); err == nil {
		t.Fatal("dangling reference accepted")
	}
	if _, err := ResolveConnectionSSH(ConnectionConfig{SSHEnabled: true, SSHProfileID: "missing", SSH: &stale}); err == nil {
		t.Fatal("dangling reference used stale credentials")
	}
	if _, err := ImportConnections(exported.Connections); err != nil {
		t.Fatal(err)
	}
	profiles, _ := ListSSHProfiles()
	if len(profiles) != 1 {
		t.Fatalf("duplicate imported profile: %d", len(profiles))
	}
	conn.SSHEnabled, conn.SSHProfileID, conn.SSH = false, "", nil
	if _, err := SaveConnection(conn); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSSHProfile(profile.ID); err != nil {
		t.Fatal(err)
	}
	profiles, _ = ListSSHProfiles()
	if len(profiles) != 0 {
		t.Fatal("account not deleted")
	}
}

func TestSSHProfileValidationAndCorruptStore(t *testing.T) {
	path := useTestConfigPath(t)
	for _, cfg := range []SSHConfig{{}, {Host: "host", User: "user", Port: -1}, {Host: "host", User: "user", Port: 65536}} {
		if _, err := SaveSSHProfile(SSHProfile{Name: "bad", SSH: cfg}); err == nil {
			t.Fatal("invalid SSH accepted")
		}
	}
	ssh := testSSHConfig()
	ssh.Port = 0
	profile, err := SaveSSHProfile(SSHProfile{Name: "default port", SSH: ssh})
	if err != nil || profile.SSH.Port != 22 {
		t.Fatal("default SSH port not applied")
	}
	corrupt := []byte("not-json")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveSSHProfile(profile); err == nil {
		t.Fatal("corrupt store accepted")
	}
	if err := DeleteSSHProfile(profile.ID); err == nil {
		t.Fatal("corrupt store deletion accepted")
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(corrupt) {
		t.Fatal("corrupt store overwritten")
	}
}

func TestSSHAccountExportImportIncludesUnusedAccountsWithoutOverwritingLocal(t *testing.T) {
	useTestConfigPath(t)
	local, err := SaveSSHProfile(SSHProfile{Name: "Shared name", SSH: testSSHConfig()})
	if err != nil {
		t.Fatal(err)
	}
	exported, err := ExportConnections()
	if err != nil || len(exported.SSHProfiles) != 1 {
		t.Fatal("unused account missing from export", err)
	}
	incoming := local
	incoming.SSH.Password = "different password"
	summary, err := ImportConfiguration([]ConnectionConfig{{Name: "imported", Host: "localhost", SSHEnabled: true, SSHProfileID: incoming.ID}}, []SSHProfile{incoming})
	if err != nil || summary.SSHAdded != 1 || summary.Added != 1 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
	profiles, _ := ListSSHProfiles()
	if len(profiles) != 2 || profiles[0].SSH.Password != local.SSH.Password || profiles[1].Name != "Shared name (2)" {
		t.Fatal("import overwrote local account or failed to rename")
	}
	connections, _ := ListConnections()
	if connections[0].SSHProfileID == local.ID || connections[0].SSH.Password != incoming.SSH.Password {
		t.Fatal("import rebound account to wrong credentials")
	}
	// Export/import again matches credentials without duplicating profiles.
	exported, _ = ExportConnections()
	if _, err := ImportConfiguration(exported.Connections, exported.SSHProfiles); err != nil {
		t.Fatal(err)
	}
	profiles, _ = ListSSHProfiles()
	if len(profiles) != 2 {
		t.Fatal("roundtrip duplicated SSH accounts")
	}
}

func TestLegacySSHImportCreatesSeparateAccountAfterSharedCredentialsChange(t *testing.T) {
	useTestConfigPath(t)
	ssh := testSSHConfig()
	conn, err := SaveConnection(ConnectionConfig{Name: "old", Host: "localhost", SSHEnabled: true, SSH: &ssh})
	if err != nil {
		t.Fatal(err)
	}
	profiles, _ := ListSSHProfiles()
	profiles[0].SSH.Password = "updated"
	if _, err := SaveSSHProfile(profiles[0]); err != nil {
		t.Fatal(err)
	}
	// Reimport old inline credentials for the same Redis connection. It must not
	// collide with its migrated account ID or mutate other users of that account.
	conn.SSHProfileID = ""
	conn.SSH = &ssh
	if _, err := ImportConnections([]ConnectionConfig{conn}); err != nil {
		t.Fatal(err)
	}
	profiles, _ = ListSSHProfiles()
	if len(profiles) != 2 || profiles[0].ID == profiles[1].ID {
		t.Fatal("legacy migration ID collision")
	}
	resolved, _ := GetConnection(conn.ID)
	if resolved.SSH.Password != ssh.Password {
		t.Fatal("legacy credentials lost")
	}
}
