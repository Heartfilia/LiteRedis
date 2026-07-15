package redis

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"LiteRedis/backend/config"
)

func TestCreateKeyAtomicallyInitializesValueAndTTL(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	req := config.CreateKeyRequest{
		Key:   "session:1",
		Type:  "hash",
		Field: "token",
		Value: "abc",
		TTL:   30,
	}
	if err := CreateKey(ctx, client, req); err != nil {
		t.Fatal(err)
	}
	value, err := client.HGet(ctx, req.Key, req.Field).Result()
	if err != nil || value != req.Value {
		t.Fatalf("hash value = %q, err=%v", value, err)
	}
	ttl, err := client.TTL(ctx, req.Key).Result()
	if err != nil || ttl != 30*time.Second {
		t.Fatalf("TTL = %s, err=%v", ttl, err)
	}
}

func TestCreateKeyExistingTargetRemainsUnchanged(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	if err := client.Set(ctx, "existing", "original", 0).Err(); err != nil {
		t.Fatal(err)
	}
	err := CreateKey(ctx, client, config.CreateKeyRequest{
		Key:         "existing",
		Type:        "string",
		StringValue: "replacement",
		TTL:         10,
	})
	if !errors.Is(err, errKeyAlreadyExists) {
		t.Fatalf("expected existing key error, got %v", err)
	}
	value, err := client.Get(ctx, "existing").Result()
	if err != nil || value != "original" {
		t.Fatalf("existing value changed to %q, err=%v", value, err)
	}
	ttl, err := client.TTL(ctx, "existing").Result()
	if err != nil || ttl != -1 {
		t.Fatalf("existing TTL changed to %s, err=%v", ttl, err)
	}
}

func TestScanClusterPageAcrossMasters(t *testing.T) {
	ctx := context.Background()
	first := testRedisClient(t)
	second := testRedisClient(t)

	for _, key := range []string{"alpha", "bravo", "charlie"} {
		if err := first.Set(ctx, key, "1", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"delta", "echo", "foxtrot"} {
		if err := second.Set(ctx, key, "1", 0).Err(); err != nil {
			t.Fatal(err)
		}

	}
	masters := []clusterScanMaster{
		{addr: "first", client: first},
		{addr: "second", client: second},
	}
	state := &clusterScanState{pattern: "*", seen: make(map[string]struct{})}

	var all []string
	for pageNumber := 0; pageNumber < 10; pageNumber++ {
		page, complete, err := scanClusterPage(ctx, state, masters, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) > 2 {
			t.Fatalf("page %d contains %d keys", pageNumber, len(page))
		}
		all = append(all, page...)
		if complete {
			break
		}
	}

	sort.Strings(all)
	want := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"}
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("keys across pages = %v, want %v", all, want)
	}
}

func TestScanClusterPagePreservesPendingKeys(t *testing.T) {
	state := &clusterScanState{
		pattern:     "*",
		masterIndex: 1,
		pending:     []string{"one", "two", "three"},
		seen:        make(map[string]struct{}),
	}
	masters := []clusterScanMaster{{addr: "already-scanned"}}

	first, complete, err := scanClusterPage(context.Background(), state, masters, 2)
	if err != nil {
		t.Fatal(err)
	}
	if complete || !reflect.DeepEqual(first, []string{"one", "two"}) {
		t.Fatalf("first page = %v complete=%v", first, complete)
	}
	second, complete, err := scanClusterPage(context.Background(), state, masters, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !complete || !reflect.DeepEqual(second, []string{"three"}) {
		t.Fatalf("second page = %v complete=%v", second, complete)
	}
}

func TestClusterScanTokenIsSingleUse(t *testing.T) {
	state := &clusterScanState{pattern: "*", seen: make(map[string]struct{})}
	token := storeClusterScanState(state)

	if got := takeClusterScanState(token); got != state {
		t.Fatal("stored cluster scan state was not returned")
	}
	if got := takeClusterScanState(token); got != nil {
		t.Fatal("cluster scan token was consumed more than once")
	}
}
