package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"LiteRedis/backend/config"
	"github.com/alicebob/miniredis/v2"
)

func TestNormalizePubSubTargets(t *testing.T) {
	got := normalizePubSubTargets([]string{" chat ", "chat", "", "  ", "news"})
	if len(got) != 2 || got[0] != "chat" || got[1] != "news" {
		t.Fatalf("unexpected targets: %v", got)
	}
	if out := normalizePubSubTargets(nil); len(out) != 0 {
		t.Fatalf("nil should stay empty, got %v", out)
	}
}

func TestPubSubReceivesMessagesAndStops(t *testing.T) {
	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mini.Close()

	port, err := strconv.Atoi(mini.Addr()[strings.LastIndex(mini.Addr(), ":")+1:])
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	app := NewApp()
	cfg := config.ConnectionConfig{ID: "c1", Name: "pubsub-test", Host: "127.0.0.1", Port: port, DB: 0}
	if err := app.manager.Connect(cfg); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer app.manager.DisconnectAll()

	if res := app.StartPubSub("c1", []string{"chat"}, nil); !res.Success {
		t.Fatalf("StartPubSub: %s", res.Message)
	}

	// 等订阅真正建立
	time.Sleep(150 * time.Millisecond)
	mini.Publish("chat", "hello")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if app.GetPubSubStatus("c1").Count > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	status := app.GetPubSubStatus("c1")
	if !status.Running || status.Count == 0 {
		t.Fatalf("expected to receive messages, got %+v", status)
	}
	if len(status.Channels) != 1 || status.Channels[0] != "chat" {
		t.Fatalf("unexpected channels: %v", status.Channels)
	}

	app.StopPubSub("c1")
	if app.GetPubSubStatus("c1").Running {
		t.Fatal("subscription should be stopped")
	}
}

func TestStartPubSubRejectsEmptyTargets(t *testing.T) {
	app := NewApp()
	res := app.StartPubSub("c1", []string{"  "}, []string{})
	if res.Success {
		t.Fatal("empty targets should be rejected")
	}
}
