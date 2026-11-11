package redis

import (
	"context"
	"math"
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

func TestInvalidImportNeverDeletesExistingKey(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	entries := []config.ExportedKey{
		{Type: "unknown"},
		{Type: "hash"},
		{Type: "list"},
		{Type: "set"},
		{Type: "zset", ZSet: []config.ZSetMember{{Member: "a", Score: math.NaN()}}},
		{Type: "stream", Stream: []config.StreamEntry{{ID: "0-0", Fields: map[string]string{"f": "v"}}}},
		{Type: "stream", Stream: []config.StreamEntry{{ID: "1-0", Fields: map[string]string{}}}},
		{Type: "string", TTL: math.MaxInt64},
	}
	for _, entry := range entries {
		t.Run(entry.Type, func(t *testing.T) {
			entry.Key = "existing"
			if err := client.Set(ctx, entry.Key, "original", time.Hour).Err(); err != nil {
				t.Fatal(err)
			}
			if written, err := WriteExportedKey(ctx, client, entry, true); err == nil || written {
				t.Fatalf("invalid entry accepted: written=%v err=%v", written, err)
			}
			if value, err := client.Get(ctx, entry.Key).Result(); err != nil || value != "original" {
				t.Fatalf("original data lost: %q %v", value, err)
			}
		})
	}
}

func TestImportDoesNotOverwriteConcurrentWriter(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	writer := redis.NewClient(&redis.Options{Addr: client.Options().Addr})
	defer writer.Close()
	client.AddHook(exportCommandHook{before: func(cmd redis.Cmder) error {
		if cmd.Name() == "eval" {
			if err := writer.Set(ctx, "k", "concurrent", 0).Err(); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}})
	written, err := WriteExportedKey(ctx, client, config.ExportedKey{Key: "k", Type: "string", String: "import", TTL: -1}, false)
	if written || err != nil {
		t.Fatalf("expected atomic skip: written=%v err=%v", written, err)
	}
	if value := writer.Get(ctx, "k").Val(); value != "concurrent" {
		t.Fatalf("concurrent target overwritten: %q", value)
	}
}

func TestWriteExportedKeyStreamAndZeroTTL(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	entry := config.ExportedKey{Key: "s", Type: "stream", TTL: -1, Stream: []config.StreamEntry{
		{ID: "1-0", Fields: map[string]string{"f": "one"}},
		{ID: "1-1", Fields: map[string]string{"f": "two"}},
	}}
	if written, err := WriteExportedKey(ctx, client, entry, false); err != nil || !written {
		t.Fatalf("write stream: %v %v", written, err)
	}
	if n := client.XLen(ctx, "s").Val(); n != 2 {
		t.Fatalf("stream length = %d", n)
	}
	entry = config.ExportedKey{Key: "expires", Type: "string", String: "v", TTL: 0}
	if _, err := WriteExportedKey(ctx, client, entry, false); err != nil {
		t.Fatal(err)
	}
	if client.Exists(ctx, entry.Key).Val() != 0 {
		t.Fatal("zero TTL was turned into a permanent key")
	}
}

func TestFormatExportCommandsRejectsLineInjection(t *testing.T) {
	output := FormatExportCommands([]config.ExportedKey{{
		Key: "key\nFLUSHALL\r\n#", Type: "stream", TTL: -1,
		Stream: []config.StreamEntry{{ID: "1-0\nFLUSHDB", Fields: map[string]string{"f": "v"}}},
	}})
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "FLUSH") || strings.ContainsRune(line, '\r') {
			t.Fatalf("injected command in export: %q", output)
		}
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
