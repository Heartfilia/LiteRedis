package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

func TestReadAndWriteExportedKey(t *testing.T) {
	ctx := context.Background()
	src := testRedisClient(t)
	dst := testRedisClient(t)

	if err := src.Set(ctx, "user:1", "alice", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := src.HSet(ctx, "cart:1", "item", "book", "qty", "2").Err(); err != nil {
		t.Fatal(err)
	}
	if err := src.RPush(ctx, "queue", "a", "b").Err(); err != nil {
		t.Fatal(err)
	}
	if err := src.SAdd(ctx, "tags", "x", "y").Err(); err != nil {
		t.Fatal(err)
	}
	if err := src.ZAdd(ctx, "rank", redis.Z{Score: 1.5, Member: "p1"}, redis.Z{Score: 2, Member: "p2"}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := src.Expire(ctx, "user:1", time.Hour).Err(); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"user:1", "cart:1", "queue", "tags", "rank"} {
		entry, err := ReadKeyForExport(ctx, src, key)
		if err != nil {
			t.Fatalf("read %s: %v", key, err)
		}
		written, err := WriteExportedKey(ctx, dst, entry, false)
		if err != nil || !written {
			t.Fatalf("write %s: written=%v err=%v", key, written, err)
		}
	}

	if value, err := dst.Get(ctx, "user:1").Result(); err != nil || value != "alice" {
		t.Fatalf("string copy = %q, err=%v", value, err)
	}
	if ttl, err := dst.TTL(ctx, "user:1").Result(); err != nil || ttl <= 0 {
		t.Fatalf("ttl not preserved: %s, err=%v", ttl, err)
	}
	if fields, err := dst.HGetAll(ctx, "cart:1").Result(); err != nil || len(fields) != 2 {
		t.Fatalf("hash copy = %#v, err=%v", fields, err)
	}
	if values, err := dst.LRange(ctx, "queue", 0, -1).Result(); err != nil || len(values) != 2 || values[0] != "a" {
		t.Fatalf("list copy = %#v, err=%v", values, err)
	}
	if members, err := dst.SCard(ctx, "tags").Result(); err != nil || members != 2 {
		t.Fatalf("set copy = %d, err=%v", members, err)
	}
	if score, err := dst.ZScore(ctx, "rank", "p2").Result(); err != nil || score != 2 {
		t.Fatalf("zset copy = %v, err=%v", score, err)
	}
}

func TestWriteExportedKeyRespectsOverwrite(t *testing.T) {
	ctx := context.Background()
	dst := testRedisClient(t)
	if err := dst.Set(ctx, "k", "original", 0).Err(); err != nil {
		t.Fatal(err)
	}

	entry := config.ExportedKey{Key: "k", Type: "string", String: "updated", TTL: -1}
	written, err := WriteExportedKey(ctx, dst, entry, false)
	if err != nil || written {
		t.Fatalf("expected skip, written=%v err=%v", written, err)
	}
	if value, _ := dst.Get(ctx, "k").Result(); value != "original" {
		t.Fatalf("value was overwritten: %q", value)
	}

	written, err = WriteExportedKey(ctx, dst, entry, true)
	if err != nil || !written {
		t.Fatalf("overwrite failed: %v", err)
	}
	if value, _ := dst.Get(ctx, "k").Result(); value != "updated" {
		t.Fatalf("value not overwritten: %q", value)
	}
}

func TestFormatExportCommands(t *testing.T) {
	entries := []config.ExportedKey{
		{Key: "user:1", Type: "string", String: "a\"b", TTL: 60},
		{Key: "cart:1", Type: "hash", Hash: map[string]string{"f": "v"}},
	}
	output := FormatExportCommands(entries)
	wants := []string{
		`SET "user:1" "a\"b"`,
		`EXPIRE "user:1" 60`,
		`HSET "cart:1" "f" "v"`,
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("commands missing %q:\n%s", want, output)
		}
	}
}
