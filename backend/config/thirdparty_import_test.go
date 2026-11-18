package config

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func makeZip(t *testing.T, name, content string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestParseARDMNewBundle(t *testing.T) {
	missingCA := filepath.Join(t.TempDir(), "missing-ca.pem")
	bundle := map[string]any{
		"connections": []any{
			map[string]any{
				"name":               "cache",
				"host":               "10.0.0.5",
				"port":               "6380",
				"auth":               "p@ss",
				"username":           "acl",
				"db":                 3,
				"cluster":            true,
				"connectionReadOnly": true,
				"groupId":            "g1",
				"sslOptions":         map[string]any{"ca": missingCA},
			},
			map[string]any{
				"name":            "sentinel",
				"host":            "127.0.0.1",
				"port":            26379,
				"sentinelOptions": map[string]any{"masterName": "mymaster"},
			},
			map[string]any{
				"name":       "mtls",
				"host":       "10.0.0.6",
				"port":       6379,
				"sslOptions": map[string]any{"cert": "/tmp/client.crt", "key": "/tmp/client.key"},
			},
			map[string]any{
				"name":       "sni",
				"host":       "10.0.0.7",
				"port":       6379,
				"sslOptions": map[string]any{"servername": "redis.example.com"},
			},
			map[string]any{
				"name": "through-ssh",
				"host": "127.0.0.1",
				"port": 6379,
				"sshOptions": map[string]any{
					"host":     "jump.example.com",
					"port":     "2222",
					"username": "ops",
					"password": "ssh-pass",
				},
			},
		},
		"groups": []any{map[string]any{"id": "g1", "name": "生产"}},
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)

	result, err := ParseThirdPartyExport(ThirdPartyPlatformARDM, []byte(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 5 || len(result.Connections) != 2 || len(result.Skipped) != 3 {
		t.Fatalf("total=%d importable=%d skipped=%d, want 5/2/3: %+v", result.Total, len(result.Connections), len(result.Skipped), result.Skipped)
	}

	cache := result.Connections[0]
	if cache.Name != "cache" || cache.Host != "10.0.0.5" || cache.Port != 6380 {
		t.Fatalf("unexpected cache connection: %+v", cache)
	}
	if cache.Password != "p@ss" || cache.Username != "acl" || !cache.ReadOnly {
		t.Fatalf("credentials/readonly were not mapped: %+v", cache)
	}
	if cache.Group != "生产" || cache.DB != 3 {
		t.Fatalf("group/db = %q/%d, want 生产/3", cache.Group, cache.DB)
	}
	if !cache.TLSEnabled || !cache.TLSSkipVerify || cache.TLSCAFile != missingCA {
		t.Fatalf("TLS mapping = %+v", cache)
	}
	if !cache.IsCluster || len(cache.ClusterAddrs) != 1 || cache.ClusterAddrs[0] != "10.0.0.5:6380" {
		t.Fatalf("cluster mapping = %+v", cache.ClusterAddrs)
	}

	ssh := result.Connections[1]
	if !ssh.SSHEnabled || ssh.SSH == nil || ssh.SSH.Host != "jump.example.com" || ssh.SSH.Port != 2222 || ssh.SSH.User != "ops" || ssh.SSH.Password != "ssh-pass" {
		t.Fatalf("SSH mapping = %+v", ssh.SSH)
	}
	if ssh.SSHProfileID != "" {
		t.Fatalf("preview must not bind a local profile ID: %q", ssh.SSHProfileID)
	}

	joinedWarnings := strings.Join(result.Warnings, "\n")
	if !strings.Contains(joinedWarnings, "跳过证书校验") || !strings.Contains(joinedWarnings, "CA 文件不存在") {
		t.Fatalf("missing TLS warnings: %v", result.Warnings)
	}
	joinedReasons := ""
	for _, item := range result.Skipped {
		joinedReasons += item.Reason + "\n"
	}
	for _, want := range []string{"Sentinel", "双向 TLS", "SNI"} {
		if !strings.Contains(joinedReasons, want) {
			t.Fatalf("skipped reasons missing %q: %v", want, result.Skipped)
		}
	}
}

func TestParseARDMLegacyArrayAndRawJSON(t *testing.T) {
	raw := []byte(`[{"name":"legacy","host":"127.0.0.1","port":6380,"auth":"pw","group":"旧分组"}]`)
	result, err := ParseThirdPartyExport(ThirdPartyPlatformARDM, raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Connections) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	cfg := result.Connections[0]
	if cfg.Port != 6380 || cfg.Group != "旧分组" || cfg.Password != "pw" {
		t.Fatalf("legacy array mapping = %+v", cfg)
	}
}

func TestParseARDMSkipsIncompleteSSH(t *testing.T) {
	raw := []byte(`{"connections":[{"name":"broken","host":"127.0.0.1","port":6379,"sshOptions":{"host":"","username":"ops"}}]}`)
	result, err := ParseThirdPartyExport(ThirdPartyPlatformARDM, raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Connections) != 0 || len(result.Skipped) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.Skipped[0].Reason, "SSH 配置不完整") {
		t.Fatalf("reason = %q", result.Skipped[0].Reason)
	}
}

func TestParseARDMRejectsInvalidInput(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":        {},
		"bad-base64":   []byte("!!!not-base64!!!"),
		"plain-text":   []byte("hello"),
		"invalid-json": []byte(base64.StdEncoding.EncodeToString([]byte("{not json"))),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseThirdPartyExport(ThirdPartyPlatformARDM, data); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParseTinyRDMZip(t *testing.T) {
	yamlBody := `
- name: 本地
  addr: 127.0.0.1
  port: 6379
  username: default
  password: secret
  last_db: 2
  mark_color: "#F75B52"
  ssl:
    enable: true
    allow_insecure: true
    cafile: /tmp/missing-ca.pem
  cluster:
    enable: true
- name: 生产
  type: group
  connections:
    - name: cache-1
      addr: 10.0.0.1
      port: 6380
      ssh:
        enable: true
        addr: jump.example.com
        port: 22
        login_type: pwd
        username: ops
        password: ssh-pass
      proxy:
        type: 2
        schema: socks5
        addr: proxy.example.com
        port: 1080
        username: proxy-user
        password: proxy-pass
    - name: sentinel
      addr: 10.0.0.2
      port: 26379
      sentinel:
        enable: true
        master: mymaster
    - name: unix-socket
      network: unix
      sock: /tmp/redis.sock
    - name: ssh-agent
      addr: 10.0.0.3
      ssh:
        enable: true
        addr: jump.example.com
        login_type: agent
        username: ops
    - name: system-proxy
      addr: 10.0.0.4
      proxy:
        type: 1
    - name: mtls
      addr: 10.0.0.5
      ssl:
        enable: true
        certfile: /tmp/client.crt
        keyfile: /tmp/client.key
    - name: custom-sni
      addr: 10.0.0.6
      ssl:
        enable: true
        sni: redis.example.com
`
	archive := makeZip(t, "connections.yaml", yamlBody)
	result, err := ParseThirdPartyExport(ThirdPartyPlatformTinyRDM, archive)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 8 || len(result.Connections) != 2 || len(result.Skipped) != 6 {
		t.Fatalf("total=%d importable=%d skipped=%d, want 8/2/6: %+v", result.Total, len(result.Connections), len(result.Skipped), result.Skipped)
	}

	local := result.Connections[0]
	if local.Name != "本地" || local.Password != "secret" || local.Username != "default" {
		t.Fatalf("local mapping = %+v", local)
	}
	if local.DB != 2 || local.IconColor != "#F75B52" {
		t.Fatalf("db/color = %d/%q", local.DB, local.IconColor)
	}
	if !local.TLSEnabled || !local.TLSSkipVerify || local.TLSCAFile != "/tmp/missing-ca.pem" {
		t.Fatalf("TLS mapping = %+v", local)
	}
	if !local.IsCluster || len(local.ClusterAddrs) != 1 || local.ClusterAddrs[0] != "127.0.0.1:6379" {
		t.Fatalf("cluster mapping = %+v", local.ClusterAddrs)
	}

	grouped := result.Connections[1]
	if grouped.Name != "cache-1" || grouped.Group != "生产" {
		t.Fatalf("group mapping = %+v", grouped)
	}
	if !grouped.SSHEnabled || grouped.SSH == nil || grouped.SSH.Host != "jump.example.com" || grouped.SSH.User != "ops" {
		t.Fatalf("SSH mapping = %+v", grouped.SSH)
	}
	if !grouped.ProxyEnabled || grouped.ProxyURL != "socks5://proxy-user:proxy-pass@proxy.example.com:1080" {
		t.Fatalf("proxy mapping = %q", grouped.ProxyURL)
	}

	reasons := ""
	for _, item := range result.Skipped {
		reasons += item.Reason + "\n"
	}
	for _, want := range []string{"Sentinel", "Unix Socket", "SSH Agent", "系统代理", "双向 TLS", "SNI"} {
		if !strings.Contains(reasons, want) {
			t.Fatalf("skipped reasons missing %q: %v", want, result.Skipped)
		}
	}
}

func TestParseTinyRDMRequiresConnectionsYAML(t *testing.T) {
	archive := makeZip(t, "other.yaml", "[]")
	if _, err := ParseThirdPartyExport(ThirdPartyPlatformTinyRDM, archive); err == nil {
		t.Fatal("expected missing connections.yaml error")
	}
	if _, err := ParseThirdPartyExport(ThirdPartyPlatformTinyRDM, []byte("not a zip")); err == nil {
		t.Fatal("expected invalid zip error")
	}
}

func TestCountThirdPartyNameConflicts(t *testing.T) {
	useTestConfigPath(t)
	if _, err := SaveConnection(ConnectionConfig{Name: "dev", Host: "127.0.0.1", Port: 6379}); err != nil {
		t.Fatal(err)
	}
	conflicts := CountThirdPartyNameConflicts([]ConnectionConfig{
		{Name: "dev", Host: "10.0.0.1", Port: 6379},
		{Name: "new", Host: "10.0.0.2", Port: 6379},
		{Name: "new", Host: "10.0.0.3", Port: 6379},
	})
	if conflicts != 2 {
		t.Fatalf("conflicts = %d, want 2", conflicts)
	}
}

func TestThirdPartyImportAddsWithoutOverwriting(t *testing.T) {
	useTestConfigPath(t)
	if _, err := SaveConnection(ConnectionConfig{Name: "cache", Host: "127.0.0.1", Port: 6379, Password: "old"}); err != nil {
		t.Fatal(err)
	}
	summary, err := ImportConfiguration([]ConnectionConfig{{
		Name: "cache", Host: "10.0.0.9", Port: 6380, Password: "new",
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Added != 1 || summary.Renamed != 1 {
		t.Fatalf("summary = %+v, want added=1 renamed=1", summary)
	}
	connections, err := ListConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 2 {
		t.Fatalf("connection count = %d, want 2", len(connections))
	}
	var original, imported *ConnectionConfig
	for i := range connections {
		switch connections[i].Name {
		case "cache":
			original = &connections[i]
		case "cache (2)":
			imported = &connections[i]
		}
	}
	if original == nil || original.Password != "old" || original.Port != 6379 {
		t.Fatalf("original connection was overwritten: %+v", original)
	}
	if imported == nil || imported.Password != "new" || imported.Port != 6380 {
		t.Fatalf("imported connection missing: %+v", imported)
	}
}

func TestFilterThirdPartyDuplicatesSkipsExistingEndpoint(t *testing.T) {
	useTestConfigPath(t)
	if _, err := SaveConnection(ConnectionConfig{Name: "existing", Host: "10.0.0.5", Port: 6379}); err != nil {
		t.Fatal(err)
	}

	kept, skipped := FilterThirdPartyDuplicates([]ConnectionConfig{
		{Name: "same", Host: "10.0.0.5", Port: 6379},
		{Name: "padded", Host: " 10.0.0.5 ", Port: 6379},
		{Name: "other-port", Host: "10.0.0.5", Port: 6380},
		{Name: "new-host", Host: "10.0.0.6", Port: 6379},
	})
	if len(kept) != 2 || len(skipped) != 2 {
		t.Fatalf("kept=%d skipped=%d, want 2/2: %+v", len(kept), len(skipped), skipped)
	}
	if kept[0].Name != "other-port" || kept[1].Name != "new-host" {
		t.Fatalf("unexpected kept connections: %+v", kept)
	}
	for _, item := range skipped {
		if !strings.Contains(item.Reason, "不会覆盖现有连接") || !strings.Contains(item.Reason, "existing") {
			t.Fatalf("skip reason = %q", item.Reason)
		}
	}
}

func TestFilterThirdPartyDuplicatesKeepsFirstWithinFile(t *testing.T) {
	useTestConfigPath(t)

	kept, skipped := FilterThirdPartyDuplicates([]ConnectionConfig{
		{Name: "first", Host: "cache.example.com", Port: 6379},
		{Name: "second", Host: "cache.example.com", Port: 6379},
		{Name: "third", Host: "CACHE.EXAMPLE.COM.", Port: 6379},
		{Name: "other-port", Host: "cache.example.com", Port: 6380},
	})
	if len(kept) != 2 || len(skipped) != 2 {
		t.Fatalf("kept=%d skipped=%d, want 2/2: %+v", len(kept), len(skipped), skipped)
	}
	if kept[0].Name != "first" || kept[1].Name != "other-port" {
		t.Fatalf("unexpected kept connections: %+v", kept)
	}
	for _, item := range skipped {
		if !strings.Contains(item.Reason, "只保留第一条") || !strings.Contains(item.Reason, "first") {
			t.Fatalf("skip reason = %q", item.Reason)
		}
	}
}

func TestFilterThirdPartyDuplicatesKeepsDifferentSSHRoutes(t *testing.T) {
	useTestConfigPath(t)
	if _, err := SaveConnection(ConnectionConfig{
		Name:       "existing-tunnel",
		Host:       "127.0.0.1",
		Port:       6379,
		SSHEnabled: true,
		SSH:        &SSHConfig{Host: "jump-a.example.com", Port: 22, User: "ops", Password: "pw"},
	}); err != nil {
		t.Fatal(err)
	}

	kept, skipped := FilterThirdPartyDuplicates([]ConnectionConfig{
		{Name: "same-tunnel", Host: "127.0.0.1", Port: 6379, SSHEnabled: true, SSH: &SSHConfig{Host: "jump-a.example.com", Port: 22, User: "ops", Password: "other"}},
		{Name: "other-jump", Host: "127.0.0.1", Port: 6379, SSHEnabled: true, SSH: &SSHConfig{Host: "jump-b.example.com", Port: 22, User: "ops", Password: "pw"}},
		{Name: "direct", Host: "127.0.0.1", Port: 6379},
	})
	if len(kept) != 2 || len(skipped) != 1 {
		t.Fatalf("kept=%d skipped=%d, want 2/1: %+v", len(kept), len(skipped), skipped)
	}
	if kept[0].Name != "other-jump" || kept[1].Name != "direct" {
		t.Fatalf("unexpected kept connections: %+v", kept)
	}
	if skipped[0].Name != "same-tunnel" || !strings.Contains(skipped[0].Reason, "existing-tunnel") {
		t.Fatalf("unexpected skipped item: %+v", skipped[0])
	}
}

func TestFilterThirdPartyDuplicatesUsesClusterNodeAddress(t *testing.T) {
	useTestConfigPath(t)
	if _, err := SaveConnection(ConnectionConfig{
		Name:         "cluster",
		IsCluster:    true,
		ClusterAddrs: []string{"10.9.9.9:7000", "10.9.9.10:7000"},
	}); err != nil {
		t.Fatal(err)
	}

	kept, skipped := FilterThirdPartyDuplicates([]ConnectionConfig{
		{Name: "node-again", Host: "10.9.9.9", Port: 7000, IsCluster: true, ClusterAddrs: []string{"10.9.9.9:7000"}},
		{Name: "different-node", Host: "10.9.9.10", Port: 7000, IsCluster: true, ClusterAddrs: []string{"10.9.9.10:7000"}},
	})
	if len(kept) != 1 || len(skipped) != 1 {
		t.Fatalf("kept=%d skipped=%d, want 1/1: %+v", len(kept), len(skipped), skipped)
	}
	if kept[0].Name != "different-node" || skipped[0].Name != "node-again" {
		t.Fatalf("unexpected split: kept=%+v skipped=%+v", kept, skipped)
	}
	if !strings.Contains(skipped[0].Reason, "10.9.9.9:7000") || !strings.Contains(skipped[0].Reason, "cluster") {
		t.Fatalf("skip reason = %q", skipped[0].Reason)
	}
}

func TestImportConfigurationCountsInlineSSHProfiles(t *testing.T) {
	useTestConfigPath(t)
	summary, err := ImportConfiguration([]ConnectionConfig{{
		Name:       "through-ssh",
		Host:       "127.0.0.1",
		Port:       6379,
		SSHEnabled: true,
		SSH:        &SSHConfig{Host: "jump.example.com", Port: 22, User: "ops", Password: "pw"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Added != 1 || summary.SSHAdded != 1 {
		t.Fatalf("summary = %+v, want added=1 ssh_added=1", summary)
	}
	profiles, err := ListSSHProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].SSH.Host != "jump.example.com" {
		t.Fatalf("SSH profiles = %+v", profiles)
	}
	connections, err := ListConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0].SSHProfileID == "" {
		t.Fatalf("connection did not reference the created SSH profile: %+v", connections)
	}
}
