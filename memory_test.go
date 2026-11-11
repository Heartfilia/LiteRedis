package main

import (
	"context"
	"net"
	"strconv"
	"testing"

	"LiteRedis/backend/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestScanMemoryReportForDBKeepsCurrentDB(t *testing.T) {
	server := miniredis.RunT(t)
	server.Set("db0", "workspace")
	server.Select(6)
	server.Set("db6", "scan target")
	host, portText, _ := net.SplitHostPort(server.Addr())
	port, _ := strconv.Atoi(portText)
	a := NewApp() // ctx=nil：不依赖 Wails runtime，仍可扫描和取消。
	t.Cleanup(a.manager.DisconnectAll)
	if err := a.manager.Connect(config.ConnectionConfig{ID: "test", Host: host, Port: port}); err != nil {
		t.Fatal(err)
	}
	report, err := a.ScanMemoryReportForDB("test", 6, "*", 100)
	if err != nil || report.DB != 6 || report.ConnID != "test" || report.ScannedKeys != 1 || report.BigKeys[0].Key != "db6" {
		t.Fatalf("wrong scan DB: %+v %v", report, err)
	}
	if current, err := a.GetConnectionDB("test"); err != nil || current != 0 {
		t.Fatalf("workspace DB changed: %d %v", current, err)
	}
	legacy, err := a.ScanMemoryReport("test", "*", 100)
	if err != nil || legacy.DB != 0 || legacy.BigKeys[0].Key != "db0" {
		t.Fatalf("legacy scan: %+v %v", legacy, err)
	}
	if _, err := a.ScanMemoryReportForDB("test", -1, "*", 100); err == nil {
		t.Fatal("negative explicit DB accepted")
	}
	if _, err := a.ScanMemoryReportForDB("test", 4096, "*", 100); err == nil {
		t.Fatal("invalid explicit DB accepted")
	}
	if _, err := a.ScanMemoryReportForDB("test", 0, "*", 100); err != nil {
		t.Fatalf("failed request left scan task locked: %v", err)
	}
}

func TestScanMemoryReportForDBUsesCurrentDBAtOpen(t *testing.T) {
	server := miniredis.RunT(t)
	server.Select(3)
	server.Set("initial", "db3")
	host, portText, _ := net.SplitHostPort(server.Addr())
	port, _ := strconv.Atoi(portText)
	a := NewApp()
	t.Cleanup(a.manager.DisconnectAll)
	if err := a.manager.Connect(config.ConnectionConfig{ID: "test", Host: host, Port: port, DB: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ScanMemoryReportForDB("test", 6, "*", 100); err != nil {
		t.Fatal(err)
	}
	client, err := a.manager.GetClient("test")
	if err != nil {
		t.Fatal(err)
	}
	if got := client.(*redis.Client).Get(context.Background(), "initial").Val(); got != "db3" {
		t.Fatal("scan altered current DB")
	}
}
