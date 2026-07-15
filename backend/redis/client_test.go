package redis

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"

	"LiteRedis/backend/config"
	"LiteRedis/backend/ssh"
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
