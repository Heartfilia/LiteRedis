package redis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"LiteRedis/backend/config"
	"github.com/redis/go-redis/v9"
)

type exportCommandHook struct {
	before         func(redis.Cmder) error
	beforePipeline func([]redis.Cmder) error
}

func (h exportCommandHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h exportCommandHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if h.beforePipeline != nil {
			if err := h.beforePipeline(cmds); err != nil {
				return err
			}
		}
		return next(ctx, cmds)
	}
}
func (h exportCommandHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if h.before != nil {
			if err := h.before(cmd); err != nil {
				return err
			}
		}
		return next(ctx, cmd)
	}
}

func seedExportTypes(t testing.TB, client *redis.Client) []string {
	t.Helper()
	ctx := context.Background()
	if err := client.Set(ctx, "string", strings.Repeat("你好\n\"", exportStringChunk), 0).Err(); err != nil {
		t.Fatal(err)
	}
	pipe := client.Pipeline()
	for i := 0; i < exportPageSize*2+17; i++ {
		value := fmt.Sprintf("value:%04d", i)
		pipe.HSet(ctx, "hash", value, "value\n\"\\")
		pipe.RPush(ctx, "list", value)
		pipe.SAdd(ctx, "set", value)
		pipe.ZAdd(ctx, "zset", redis.Z{Member: value, Score: float64(i) / 2})
		pipe.XAdd(ctx, &redis.XAddArgs{Stream: "stream", ID: fmt.Sprintf("%d-0", i+1), Values: map[string]string{"f": value}})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	return []string{"string", "hash", "list", "set", "zset", "stream"}
}

func TestExportPagedJSONRoundTrip(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	keys := seedExportTypes(t, client)
	counts := map[string]int{}
	client.AddHook(exportCommandHook{before: func(cmd redis.Cmder) error {
		counts[cmd.Name()]++
		switch cmd.Name() {
		case "get", "hgetall", "smembers":
			return fmt.Errorf("unbounded command %s", cmd.Name())
		case "lrange", "zrange":
			args := cmd.Args()
			start, _ := strconv.ParseInt(fmt.Sprint(args[2]), 10, 64)
			end, _ := strconv.ParseInt(fmt.Sprint(args[3]), 10, 64)
			if end < start || end-start >= exportPageSize {
				return fmt.Errorf("unbounded range: %v", args)
			}
		case "xrange":
			if !strings.Contains(strings.ToLower(cmd.String()), "count") {
				return errors.New("unbounded XRANGE")
			}
		}
		return nil
	}})
	var out bytes.Buffer
	if err := ExportKeysToWriter(ctx, client, keys, "json", &out, 0); err != nil {
		t.Fatal(err)
	}
	var entries []config.ExportedKey
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(keys) {
		t.Fatalf("entries = %d", len(entries))
	}
	dst := testRedisClient(t)
	for _, entry := range entries {
		if written, err := WriteExportedKey(ctx, dst, entry, false); err != nil || !written {
			t.Fatalf("import %s: %v %v", entry.Key, written, err)
		}
		copy, err := ReadKeyForExport(ctx, dst, entry.Key)
		if err != nil || !reflect.DeepEqual(entry, copy) {
			t.Fatalf("round trip mismatch for %s: %v", entry.Key, err)
		}
	}
	for _, name := range []string{"getrange", "lrange", "zrange", "xrange"} {
		if counts[name] < 3 {
			t.Fatalf("%s did not use multiple pages: %v", name, counts)
		}
	}
	if counts["hscan"] == 0 || counts["sscan"] == 0 {
		t.Fatalf("missing scans: %v", counts)
	}
}

func TestExportCommandWriterMatchesExistingFormat(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	keys := seedExportTypes(t, client)
	var entries []config.ExportedKey
	for _, key := range keys {
		entry, err := ReadKeyForExport(ctx, client, key)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	var out bytes.Buffer
	if err := ExportKeysToWriter(ctx, client, keys, "commands", &out, 0); err != nil {
		t.Fatal(err)
	}
	if out.String() != FormatExportCommands(entries) {
		t.Fatal("streamed commands changed export format")
	}
}

func TestExportRejectsOversizedStringBeforeReadingValue(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	if err := client.Set(ctx, "large", strings.Repeat("v", 1024), 0).Err(); err != nil {
		t.Fatal(err)
	}
	client.AddHook(exportCommandHook{before: func(cmd redis.Cmder) error {
		if cmd.Name() == "get" || cmd.Name() == "getrange" {
			t.Fatal("oversized string read before size check")
		}
		return nil
	}})
	if _, err := readKeyForExport(ctx, client, "large", 100, 10); err == nil {
		t.Fatal("oversized string accepted")
	}
}

func TestExportKeyBudgets(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	keys := seedExportTypes(t, client)
	for _, key := range keys[1:] {
		t.Run(key, func(t *testing.T) {
			if _, err := readKeyForExport(ctx, client, key, 32<<20, 10); err == nil {
				t.Fatal("item budget ignored")
			}
			if _, err := readKeyForExport(ctx, client, key, 100, ExportMaxKeyItems); err == nil {
				t.Fatal("byte budget ignored")
			}
		})
	}
}

func TestExportEncodedFileBudget(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	if err := client.Set(ctx, "s", strings.Repeat("\x00", 50), 0).Err(); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"json", "commands"} {
		var full bytes.Buffer
		if err := ExportKeysToWriter(ctx, client, []string{"s"}, format, &full, 4096); err != nil {
			t.Fatal(err)
		}
		var limited bytes.Buffer
		if err := ExportKeysToWriter(ctx, client, []string{"s"}, format, &limited, int64(full.Len()-1)); err == nil {
			t.Fatal("encoded budget ignored")
		}
		if limited.Len() >= full.Len() {
			t.Fatal("wrote beyond byte budget")
		}
	}
}

func TestExportCancelledBetweenPages(t *testing.T) {
	client := testRedisClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < exportPageSize*2; i++ {
		if err := client.RPush(ctx, "list", "v").Err(); err != nil {
			t.Fatal(err)
		}
	}
	pages := 0
	client.AddHook(exportCommandHook{before: func(cmd redis.Cmder) error {
		if cmd.Name() == "lrange" {
			pages++
			cancel()
		}
		return nil
	}})
	err := ExportKeysToWriter(ctx, client, []string{"list"}, "json", io.Discard, 0)
	if !errors.Is(err, context.Canceled) || pages != 1 {
		t.Fatalf("cancel = %v pages=%d", err, pages)
	}
}

func TestExportWriterFailureStopsReading(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	keys := seedExportTypes(t, client)
	want := errors.New("disk full")
	if err := ExportKeysToWriter(ctx, client, keys, "json", failingExportWriter{want}, 0); !errors.Is(err, want) {
		t.Fatalf("writer error = %v", err)
	}
}

func TestExportWritesEachKeyBeforeReadingNext(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	for _, key := range []string{"first", "second"} {
		if err := client.Set(ctx, key, "value", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	want := errors.New("disk full after header")
	w := &exportFailAfterHeader{err: want}
	client.AddHook(exportCommandHook{before: func(cmd redis.Cmder) error {
		if len(cmd.Args()) > 1 && fmt.Sprint(cmd.Args()[1]) == "second" {
			t.Fatal("read another key after failed output")
		}
		return nil
	}})
	if err := ExportKeysToWriter(ctx, client, []string{"first", "second"}, "json", w, 0); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

func TestExportRejectsMetadataErrors(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	if err := client.RPush(ctx, "list", "v").Err(); err != nil {
		t.Fatal(err)
	}
	client.AddHook(exportCommandHook{beforePipeline: func(cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			if cmd.Name() == "llen" {
				return errors.New("NOPERM length command denied")
			}
		}
		return nil
	}})
	if _, err := ReadKeyForExport(ctx, client, "list"); err == nil {
		t.Fatal("metadata errors silently treated as empty list")
	}
}

type exportFailAfterHeader struct {
	err    error
	writes int
}

func (w *exportFailAfterHeader) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		return len(p), nil
	}
	return 0, w.err
}

func TestExportRejectsCollectionChangedDuringRead(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	for i := 0; i < exportPageSize+1; i++ {
		if err := client.RPush(ctx, "list", "v").Err(); err != nil {
			t.Fatal(err)
		}
	}
	writer := redis.NewClient(&redis.Options{Addr: client.Options().Addr})
	defer writer.Close()
	client.AddHook(exportCommandHook{before: func(cmd redis.Cmder) error {
		if cmd.Name() == "lrange" {
			return writer.RPush(ctx, "list", "new").Err()
		}
		return nil
	}})
	if _, err := ReadKeyForExport(ctx, client, "list"); err == nil {
		t.Fatal("concurrently growing list exported as a snapshot")
	}
}

func TestExportScanDeduplicatesRepeatedFieldsAndMembers(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"hash", "set"} {
		t.Run(kind, func(t *testing.T) {
			client := testRedisClient(t)
			if kind == "hash" {
				if err := client.HSet(ctx, "key", "a", "1", "b", "2").Err(); err != nil {
					t.Fatal(err)
				}
			} else if err := client.SAdd(ctx, "key", "a", "b").Err(); err != nil {
				t.Fatal(err)
			}
			pages := 0
			client.AddHook(exportScanHook{respond: func(cmd *redis.ScanCmd) {
				pages++
				values, cursor := []string{"a"}, uint64(1)
				if kind == "hash" {
					values = []string{"a", "1"}
				}
				if pages == 2 {
					values, cursor = []string{"a", "b"}, 0
					if kind == "hash" {
						values = []string{"a", "1", "b", "2"}
					}
				}
				cmd.SetVal(values, cursor)
			}})
			entry, err := readKeyForExport(ctx, client, "key", 1024, 2)
			if err != nil {
				t.Fatal(err)
			}
			if pages != 2 || (kind == "hash" && len(entry.Hash) != 2) || (kind == "set" && len(entry.Set) != 2) {
				t.Fatalf("duplicate scan results: %+v, pages=%d", entry, pages)
			}
		})
	}
}

type exportScanHook struct{ respond func(*redis.ScanCmd) }

func (h exportScanHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h exportScanHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h exportScanHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "hscan" || cmd.Name() == "sscan" {
			h.respond(cmd.(*redis.ScanCmd))
			return nil
		}
		return next(ctx, cmd)
	}
}

type failingExportWriter struct{ err error }

func (w failingExportWriter) Write([]byte) (int, error) { return 0, w.err }

func TestExportDetectsDeletedKey(t *testing.T) {
	client := testRedisClient(t)
	if _, err := ReadKeyForExport(context.Background(), client, "missing"); err == nil {
		t.Fatal("missing key exported")
	}
}
