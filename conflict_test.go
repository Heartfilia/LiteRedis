package main

import (
	"context"
	"testing"
	"time"
)

func TestGetEditConflictUsesExplicitDBWithoutChangingWorkspace(t *testing.T) {
	a, mini := connectedWriteTestApp(t)
	mini.DB(4).Set("key", "workspace")
	mini.DB(7).Set("key", "editing database")
	mini.DB(7).HSet("hash", "field", "field value")
	mini.DB(7).SetTTL("key", time.Minute)
	if err := a.manager.SelectDB("test", 4); err != nil {
		t.Fatal(err)
	}
	s, err := a.GetEditConflict("test", 7, "key", "", false)
	if err != nil || !s.Exists || s.Type != "string" || s.Value != "editing database" {
		t.Fatalf("wrong DB: %+v %v", s, err)
	}
	h, err := a.GetEditConflict("test", 7, "hash", "field", true)
	if err != nil || !h.Exists || h.Value != "field value" {
		t.Fatalf("hash wrong DB: %+v %v", h, err)
	}
	_, db, err := a.manager.GetConnectionState("test")
	if err != nil || db != 4 {
		t.Fatalf("workspace changed: %d %v", db, err)
	}
	if value, _ := mini.DB(4).Get("key"); value != "workspace" {
		t.Fatal("workspace key changed")
	}
	if value, _ := mini.DB(7).Get("key"); value != "editing database" || mini.DB(7).TTL("key") != time.Minute {
		t.Fatal("snapshot changed data or TTL")
	}
}

func TestGetEditConflictValidationAndCancellation(t *testing.T) {
	a, _ := connectedWriteTestApp(t)
	for _, tc := range []struct {
		conn       string
		db         int
		key, field string
		hash       bool
	}{
		{"", 0, "key", "", false}, {"test", -1, "key", "", false}, {"test", 0, "", "", false}, {"test", 0, "key", "unexpected", false}, {"missing", 0, "key", "", false},
	} {
		if _, err := a.GetEditConflict(tc.conn, tc.db, tc.key, tc.field, tc.hash); err == nil {
			t.Fatalf("accepted invalid parameters: %+v", tc)
		}
	}
	// Empty hash field names are valid Redis inputs and must remain readable.
	if _, err := a.GetEditConflict("test", 0, "hash", "", true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.ctx = ctx
	if _, err := a.GetEditConflict("test", 0, "key", "", false); err == nil {
		t.Fatal("ignored cancellation")
	}
}
