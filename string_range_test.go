package main

import (
	"context"
	"errors"
	"testing"
	"time"

	redisbackend "LiteRedis/backend/redis"
)

func TestGetStringRangePreviewExplicitDBReadOnly(t *testing.T) {
	a, mini := connectedWriteTestApp(t)
	mini.DB(4).Set("key", "workspace")
	mini.DB(7).Set("key", "preview database")
	mini.DB(7).SetTTL("key", time.Minute)
	if err := a.manager.SelectDB("test", 4); err != nil {
		t.Fatal(err)
	}
	page, err := a.GetStringRangePreview("test", 7, "key", 8)
	if err != nil || page.Value != "database" || page.Offset != 8 || page.NextOffset != 16 || page.HasMore || page.Encoding != "utf-8" {
		t.Fatalf("wrong DB/range: %+v %v", page, err)
	}
	_, db, err := a.manager.GetConnectionState("test")
	if err != nil || db != 4 {
		t.Fatalf("workspace DB changed: %d %v", db, err)
	}
	if value, _ := mini.DB(4).Get("key"); value != "workspace" {
		t.Fatal("workspace data changed")
	}
	if value, _ := mini.DB(7).Get("key"); value != "preview database" || mini.DB(7).TTL("key") != time.Minute {
		t.Fatal("preview changed value or TTL")
	}
}

func TestGetStringRangePreviewValidationAndCancellation(t *testing.T) {
	a, mini := connectedWriteTestApp(t)
	mini.DB(0).Set("key", "value")
	for _, tc := range []struct {
		conn   string
		db     int
		key    string
		offset int64
	}{
		{"", 0, "key", 0}, {" ", 0, "key", 0}, {"test", -1, "key", 0},
		{"test", 0, "", 0}, {"test", 0, "key", -1},
		{"test", 0, "key", redisbackend.MaxStringRangeOffset + 1},
		{"missing", 0, "key", 0}, {"test", 0, "key", 6},
	} {
		if _, err := a.GetStringRangePreview(tc.conn, tc.db, tc.key, tc.offset); err == nil {
			t.Fatalf("accepted invalid parameters: %+v", tc)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.ctx = ctx
	if _, err := a.GetStringRangePreview("test", 0, "key", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", err)
	}
	a.ctx = nil
	if _, err := a.GetStringRangePreview("test", 0, "key", 0); err != nil {
		t.Fatalf("nil app context fallback: %v", err)
	}
	if _, err := (&App{}).GetStringRangePreview("test", 0, "key", 0); err == nil {
		t.Fatal("accepted unavailable manager")
	}
}
