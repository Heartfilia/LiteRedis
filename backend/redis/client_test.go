package redis

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"

	"LiteRedis/backend/config"
	"LiteRedis/backend/ssh"
	goredis "github.com/redis/go-redis/v9"
)

func TestBuildActiveRedisOptionsRestoresProxy(t *testing.T) {
	conn := &activeConn{cfg: config.ConnectionConfig{
		Host:         "redis.internal",
		Port:         6380,
		Password:     "secret",
		ProxyEnabled: true,
		ProxyURL:     "socks5://127.0.0.1:1080",
	}}

	opts, err := buildActiveRedisOptions(conn, 6)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Addr != "redis.internal:6380" || opts.DB != 6 || opts.Password != "secret" {
		t.Fatalf("unexpected redis options: addr=%q db=%d password=%q", opts.Addr, opts.DB, opts.Password)
	}
	if opts.Dialer == nil {
		t.Fatal("proxy dialer was not restored")
	}
}

func TestReadClientForDBDoesNotChangeWorkspaceDB(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	if err := client.Set(ctx, "workspace", "db0", 0).Err(); err != nil {
		t.Fatal(err)
	}
	manager := NewClientManager()
	manager.clients["test"] = &activeConn{client: client}
	reader, db, release, err := manager.NewReadClientForDB("test", 6)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if db != 6 || reader == client {
		t.Fatal("expected independent DB 6 client")
	}
	if err := reader.Set(ctx, "other", "db6", 0).Err(); err != nil {
		t.Fatal(err)
	}
	report, err := ScanMemory(ctx, reader, "*", 100, nil)
	if err != nil || report.ScannedKeys != 1 || report.BigKeys[0].Key != "other" {
		t.Fatalf("wrong DB scanned: %+v %v", report, err)
	}
	if exists := client.Exists(ctx, "other").Val(); exists != 0 {
		t.Fatal("scan changed workspace DB")
	}
	active, err := manager.GetClient("test")
	if err != nil || active != client {
		t.Fatal("workspace client replaced")
	}
	_, current, _ := manager.GetConnectionState("test")
	if current != 0 || client.Get(ctx, "workspace").Val() != "db0" {
		t.Fatal("workspace DB changed")
	}
	release()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("release closed workspace client: %v", err)
	}
	currentReader, currentDB, closeCurrent, err := manager.NewReadClientForDB("test", -1)
	if err != nil {
		t.Fatal(err)
	}
	defer closeCurrent()
	if currentDB != 0 || currentReader.Get(ctx, "workspace").Val() != "db0" {
		t.Fatal("legacy current DB scan changed")
	}
}

func TestOperationClientIsPinnedAndDoesNotRetry(t *testing.T) {
	client := testRedisClient(t)
	manager := NewClientManager()
	manager.clients["test"] = &activeConn{client: client}
	operation, release, err := manager.NewOperationClientForDB("test", 6)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	opts := operation.(*goredis.Client).Options()
	if opts.DB != 6 || opts.MaxRetries != 0 || operation == client {
		t.Fatal("write client is shared, retries enabled or wrong DB")
	}
	if err := operation.Set(context.Background(), "pinned", "DB6", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if client.Exists(context.Background(), "pinned").Val() != 0 {
		t.Fatal("write changed workspace DB")
	}
	cluster := goredis.NewClusterClient(&goredis.ClusterOptions{Addrs: []string{"127.0.0.1:1"}, MaxRetries: 1})
	defer cluster.Close()
	manager.clients["cluster"] = &activeConn{client: cluster, cfg: config.ConnectionConfig{IsCluster: true}}
	writer, closeWriter, err := manager.NewOperationClientForDB("cluster", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer closeWriter()
	clusterOpts := writer.(*goredis.ClusterClient).Options()
	if writer == cluster || clusterOpts.MaxRetries != -1 || clusterOpts.MaxRedirects != 0 {
		t.Fatal("cluster write replay enabled")
	}
}

func TestReadClientForDBPreservesTransportOptions(t *testing.T) {
	dialer := (&net.Dialer{}).DialContext
	client := goredis.NewClient(&goredis.Options{
		Addr: "redis.internal:6380", Username: "acl", Password: "secret", DB: 3,
		Dialer: dialer, TLSConfig: &tls.Config{ServerName: "redis.internal", MinVersion: tls.VersionTLS12},
	})
	defer client.Close()
	manager := NewClientManager()
	manager.clients["test"] = &activeConn{client: client, currentDB: 3}
	reader, db, release, err := manager.NewReadClientForDB("test", 7)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	opts := reader.(*goredis.Client).Options()
	if db != 7 || opts.DB != 7 || opts.Addr != client.Options().Addr || opts.Dialer == nil || opts.Username != "acl" || opts.Password != "secret" {
		t.Fatalf("lost transport options: %+v", opts)
	}
	if opts.TLSConfig == client.Options().TLSConfig || opts.TLSConfig.ServerName != "redis.internal" {
		t.Fatal("TLS config not cloned")
	}
}

func TestReadClientForDBRejectsInvalidDBAndRestrictsCluster(t *testing.T) {
	manager := NewClientManager()
	client := testRedisClient(t)
	manager.clients["single"] = &activeConn{client: client}
	for _, db := range []int{-2, 4096} {
		if _, _, _, err := manager.NewReadClientForDB("single", db); err == nil {
			t.Fatalf("accepted DB %d", db)
		}
	}
	if _, _, _, err := manager.NewReadClientForDB("missing", 0); err == nil {
		t.Fatal("missing connection accepted")
	}
	cluster := goredis.NewClusterClient(&goredis.ClusterOptions{Addrs: []string{client.Options().Addr}})
	defer cluster.Close()
	manager.clients["cluster"] = &activeConn{client: cluster, cfg: config.ConnectionConfig{IsCluster: true}}
	if _, _, _, err := manager.NewReadClientForDB("cluster", 1); err == nil {
		t.Fatal("cluster accepted DB 1")
	}
	reader, db, release, err := manager.NewReadClientForDB("cluster", 0)
	if err != nil || db != 0 || reader != cluster {
		t.Fatalf("cluster DB 0: %v", err)
	}
	release()
}

func TestJoinHostPortSupportsIPv6(t *testing.T) {
	if got := joinHostPort("2001:db8::1", 6379); got != "[2001:db8::1]:6379" {
		t.Fatalf("IPv6 address = %q", got)
	}
	if got := joinHostPort("localhost", 6380); got != "127.0.0.1:6380" {
		t.Fatalf("localhost address = %q", got)
	}
}

func TestBuildActiveRedisOptionsPrefersSSHForward(t *testing.T) {
	conn := &activeConn{
		cfg: config.ConnectionConfig{
			Host:         "redis.internal",
			Port:         6379,
			ProxyEnabled: true,
			ProxyURL:     "socks5://127.0.0.1:1080",
		},
		sshForward:    &ssh.LocalForward{},
		forwardedAddr: "127.0.0.1:49152",
	}

	opts, err := buildActiveRedisOptions(conn, 3)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Addr != "127.0.0.1:49152" || opts.DB != 3 {
		t.Fatalf("unexpected forwarded options: addr=%q db=%d", opts.Addr, opts.DB)
	}
	if opts.Dialer != nil {
		t.Fatal("local SSH forward should not install a second dialer")
	}
}

func TestBuildActiveRedisOptionsRejectsInvalidProxy(t *testing.T) {
	conn := &activeConn{cfg: config.ConnectionConfig{
		Host:         "redis.internal",
		Port:         6379,
		ProxyEnabled: true,
		ProxyURL:     "ftp://127.0.0.1:21",
	}}

	if _, err := buildActiveRedisOptions(conn, 0); err == nil {
		t.Fatal("invalid proxy scheme was accepted")
	}
}

func TestRedactProxyURLRemovesCredentials(t *testing.T) {
	raw := "http://alice:secret@127.0.0.1:7890/private?token=hidden"
	redacted := redactProxyURL(raw)
	if redacted != "http://***@127.0.0.1:7890" {
		t.Fatalf("redacted URL = %q", redacted)
	}
	if safeErr := redactProxyError(errors.New("parse "+raw), raw); strings.Contains(safeErr.Error(), "secret") || strings.Contains(safeErr.Error(), "hidden") {
		t.Fatalf("proxy error leaked credentials: %v", safeErr)
	}
}

func TestApplyConnectionConfigInvalidatesOnlyTransportChanges(t *testing.T) {
	client := testRedisClient(t)
	manager := NewClientManager()
	original := config.ConnectionConfig{
		ID:   "conn-1",
		Name: "Original",
		Host: "127.0.0.1",
		Port: 6379,
	}
	manager.clients[original.ID] = &activeConn{client: client, cfg: original}

	metadataOnly := original
	metadataOnly.Name = "Renamed"
	metadataOnly.Group = "Production"
	metadataOnly.AllowClusterScan = true
	if manager.ApplyConnectionConfig(metadataOnly) {
		t.Fatal("metadata-only change disconnected the client")
	}
	if !manager.IsConnected(original.ID) {
		t.Fatal("metadata-only change removed the client")
	}

	transportChange := metadataOnly
	transportChange.Host = "redis.internal"
	if !manager.ApplyConnectionConfig(transportChange) {
		t.Fatal("transport change did not invalidate the client")
	}
	if manager.IsConnected(original.ID) {
		t.Fatal("transport change left the stale client active")
	}
}

func TestConnectFailureKeepsExistingClient(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, closedPortText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	closedPort, err := strconv.Atoi(closedPortText)
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	manager := NewClientManager()
	manager.clients["conn-1"] = &activeConn{
		client: client,
		cfg: config.ConnectionConfig{
			ID:   "conn-1",
			Host: "127.0.0.1",
			Port: 6379,
		},
	}

	if err := manager.Connect(config.ConnectionConfig{ID: "conn-1", Host: "127.0.0.1", Port: closedPort}); err == nil {
		t.Fatal("connection to a closed port unexpectedly succeeded")
	}
	active, err := manager.GetClient("conn-1")
	if err != nil {
		t.Fatalf("existing client was removed after failed replacement: %v", err)
	}
	if active != client {
		t.Fatal("failed replacement changed the active client")
	}
	if err := active.Ping(ctx).Err(); err != nil {
		t.Fatalf("existing client was closed after failed replacement: %v", err)
	}
}

func TestSelectDBReplacesClientAfterSuccessfulPing(t *testing.T) {
	client := testRedisClient(t)
	host, portText, err := net.SplitHostPort(client.Options().Addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewClientManager()
	manager.clients["conn-1"] = &activeConn{
		client: client,
		cfg: config.ConnectionConfig{
			ID:   "conn-1",
			Host: host,
			Port: port,
		},
	}

	if err := manager.SelectDB("conn-1", 6); err != nil {
		t.Fatalf("select DB: %v", err)
	}
	_, db, err := manager.GetConnectionState("conn-1")
	if err != nil {
		t.Fatal(err)
	}
	if db != 6 {
		t.Fatalf("current DB = %d, want 6", db)
	}
}
