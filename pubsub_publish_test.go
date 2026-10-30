package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"LiteRedis/backend/config"
	"github.com/alicebob/miniredis/v2"
)

func TestPublishMessageReachesSubscriber(t *testing.T) {
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
	cfg := config.ConnectionConfig{ID: "c1", Name: "publish-test", Host: "127.0.0.1", Port: port, DB: 0}
	if err := app.manager.Connect(cfg); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer app.manager.DisconnectAll()

	if res := app.StartPubSub("c1", []string{"chat"}, nil); !res.Success {
		t.Fatalf("StartPubSub: %s", res.Message)
	}
	defer app.StopPubSub("c1")
	time.Sleep(150 * time.Millisecond)

	receivers, err := app.PublishMessage("c1", "chat", "hi from panel")
	if err != nil {
		t.Fatalf("PublishMessage: %v", err)
	}
	if receivers < 1 {
		t.Fatalf("expected at least one receiver, got %d", receivers)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if app.GetPubSubStatus("c1").Count > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if app.GetPubSubStatus("c1").Count == 0 {
		t.Fatal("published message was not received by the subscription")
	}

	if _, err := app.PublishMessage("c1", "   ", "x"); err == nil {
		t.Fatal("empty channel should be rejected")
	}
}
