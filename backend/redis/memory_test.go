package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func seedMemoryFixtures(t *testing.T, ctx context.Context, client *goredis.Client) {
	t.Helper()
	if err := client.Set(ctx, "user:1", "a", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, "user:2", "bb", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, "session:1", "cc", time.Hour).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.HSet(ctx, "cart:1", "item", "x").Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, "plain", "z", 0).Err(); err != nil {
		t.Fatal(err)
	}
}

func TestScanMemoryAggregatesPrefixAndType(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	seedMemoryFixtures(t, ctx, client)

	report, err := ScanMemory(ctx, client, "*", 1000, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if report.ScannedKeys != 5 {
		t.Fatalf("scanned = %d, want 5", report.ScannedKeys)
	}
	if report.Truncated {
		t.Fatal("small scan was marked truncated")
	}
	if report.NoTTLKeys != 4 {
		t.Fatalf("noTTL = %d, want 4", report.NoTTLKeys)
	}
	if report.SizeUnavailable {
		t.Fatal("size should be available with miniredis")
	}
	if report.TotalBytes <= 0 {
		t.Fatalf("total bytes = %d", report.TotalBytes)
	}

	prefixes := map[string]int64{}
	for _, item := range report.Prefixes {
		prefixes[item.Prefix] = item.Keys
	}
	if prefixes["user"] != 2 || prefixes["session"] != 1 || prefixes["cart"] != 1 || prefixes[MemoryNoPrefixLabel] != 1 {
		t.Fatalf("prefix aggregation = %#v", prefixes)
	}

	types := map[string]int64{}
	for _, item := range report.Types {
		types[item.Type] = item.Keys
	}
	if types["string"] != 4 || types["hash"] != 1 {
		t.Fatalf("type aggregation = %#v", types)
	}
	if len(report.BigKeys) != 5 {
		t.Fatalf("big keys = %d, want 5", len(report.BigKeys))
	}
	for i := 1; i < len(report.BigKeys); i++ {
		if report.BigKeys[i-1].Size < report.BigKeys[i].Size {
			t.Fatalf("big keys are not sorted desc: %#v", report.BigKeys)
		}
	}
}

func TestScanMemoryRespectsPatternAndLimit(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	seedMemoryFixtures(t, ctx, client)

	filtered, err := ScanMemory(ctx, client, "user:*", 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filtered.ScannedKeys != 2 {
		t.Fatalf("filtered scanned = %d, want 2", filtered.ScannedKeys)
	}

	limited, err := ScanMemory(ctx, client, "*", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !limited.Truncated {
		t.Fatal("limit was not reported as truncated")
	}
	if limited.ScannedKeys > memoryScanBatchSize {
		t.Fatalf("limit ignored: scanned = %d", limited.ScannedKeys)
	}
}

func TestScanMemoryStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := testRedisClient(t)
	seedMemoryFixtures(t, ctx, client)

	cancel()
	_, err := ScanMemory(ctx, client, "*", 1000, nil)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("cancelled scan err = %v", err)
	}
}

func TestScanMemoryReportsProgress(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	seedMemoryFixtures(t, ctx, client)

	var lastScanned int64
	_, err := ScanMemory(ctx, client, "*", 1000, func(scanned, total int64) {
		lastScanned = scanned
	})
	if err != nil {
		t.Fatal(err)
	}
	if lastScanned == 0 {
		t.Fatal("progress callback was never called")
	}
}

func TestParseSerializedLength(t *testing.T) {
	raw := "Value at:0x7f8 refcount:1 encoding:embstr serializedlength:37 lru:0 lru_seconds_idle:3"
	value, ok := parseSerializedLength(raw)
	if !ok || value != 37 {
		t.Fatalf("parsed = %d, ok=%v", value, ok)
	}
	if _, ok := parseSerializedLength("Value at:0x1 refcount:1"); ok {
		t.Fatal("missing serializedlength was accepted")
	}
	if _, ok := parseSerializedLength("serializedlength:abc"); ok {
		t.Fatal("non numeric serializedlength was accepted")
	}
}

func TestScanMemoryReportsSizeSource(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	seedMemoryFixtures(t, ctx, client)

	report, err := ScanMemory(ctx, client, "*", 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.SizeSource != SizeSourceMemory {
		t.Fatalf("size source = %q, want %q", report.SizeSource, SizeSourceMemory)
	}
	for _, item := range report.BigKeys {
		if item.Len != 0 {
			t.Fatalf("len should stay 0 in memory mode: %#v", item)
		}
	}
}

func TestAddBatchFallsBackToElementCount(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	seedMemoryFixtures(t, ctx, client)

	acc := newMemoryAccumulator()
	if err := acc.addBatch(ctx, client, []string{"user:1", "cart:1"}, sizeModeLength); err != nil {
		t.Fatal(err)
	}
	acc.scanned = 2
	report := acc.build()

	lengths := map[string]int64{}
	for _, item := range report.BigKeys {
		lengths[item.Key] = item.Len
	}
	if lengths["user:1"] != 1 {
		t.Fatalf("string length = %d, want 1", lengths["user:1"])
	}
	if lengths["cart:1"] != 1 {
		t.Fatalf("hash length = %d, want 1", lengths["cart:1"])
	}
	if report.TotalBytes != 0 {
		t.Fatalf("bytes should stay 0 in length mode, got %d", report.TotalBytes)
	}
	if report.NoTTLKeys != 2 {
		t.Fatalf("noTTL = %d, want 2", report.NoTTLKeys)
	}
}
