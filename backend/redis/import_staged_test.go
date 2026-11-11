package redis

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"LiteRedis/backend/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	goredis "github.com/redis/go-redis/v9"
)

func TestStagedImportFailurePreservesTargetAndTTL(t *testing.T) {
	for _, denied := range []string{"HMSET", "PEXPIRE", "PERSIST", "EXPIRE", "RENAME"} {
		t.Run(denied, func(t *testing.T) {
			mini := miniredis.RunT(t)
			client := goredis.NewClient(&goredis.Options{Addr: mini.Addr(), MaxRetries: -1})
			defer client.Close()
			ctx := context.Background()
			client.Set(ctx, "target", "original", time.Hour)
			mini.Server().SetPreHook(func(peer *server.Peer, command string, _ ...string) bool {
				if command == denied {
					peer.WriteError("NOPERM test denied " + denied)
					return true
				}
				return false
			})
			entry := config.ExportedKey{Key: "target", Type: "hash", Hash: map[string]string{"f": "new"}, TTL: -1}
			if denied == "EXPIRE" {
				entry.TTL = 60
			}
			written, err := WriteExportedKey(ctx, client, entry, true)
			if written || err == nil {
				t.Fatalf("denied command accepted: written=%v err=%v", written, err)
			}
			mini.Server().SetPreHook(nil)
			if client.Get(ctx, "target").Val() != "original" || client.TTL(ctx, "target").Val() <= 0 {
				t.Fatal("failed import damaged original/TTL")
			}
			if keys := client.Keys(ctx, "*").Val(); len(keys) != 1 {
				t.Fatalf("staging key leaked: %v", keys)
			}
		})
	}
}

func TestStagedImportCleanupFailureIsExplicit(t *testing.T) {
	mini := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mini.Addr(), MaxRetries: -1})
	defer client.Close()
	ctx := context.Background()
	client.Set(ctx, "target", "original", time.Hour)
	mini.Server().SetPreHook(func(peer *server.Peer, command string, _ ...string) bool {
		if command == "RENAME" || command == "DEL" {
			peer.WriteError("NOPERM test denied")
			return true
		}
		return false
	})
	_, err := WriteExportedKey(ctx, client, config.ExportedKey{Key: "target", Type: "string", String: "new", TTL: 60}, true)
	if err == nil || !strings.Contains(err.Error(), "staging cleanup failed:") {
		t.Fatalf("cleanup failure hidden: %v", err)
	}
	mini.Server().SetPreHook(nil)
	if client.Get(ctx, "target").Val() != "original" {
		t.Fatal("original overwritten")
	}
	for _, key := range client.Keys(ctx, "*").Val() {
		if key != "target" && client.PTTL(ctx, key).Val() <= 0 {
			t.Fatal("abandoned staging data has no TTL")
		}
	}
}

func TestStagedImportRuntimeFailureAfterConstructionPreservesOriginal(t *testing.T) {
	mini := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mini.Addr(), MaxRetries: -1})
	defer client.Close()
	ctx := context.Background()
	client.Set(ctx, "target", "original", time.Hour)
	calls := 0
	mini.Server().SetPreHook(func(peer *server.Peer, command string, _ ...string) bool {
		if command == "HMSET" {
			calls++
			if calls == 2 {
				peer.WriteError("OOM simulated allocation failure")
				return true
			}
		}
		return false
	})
	fields := make(map[string]string)
	for i := 0; i < exportPageSize+1; i++ {
		fields[strconv.Itoa(i)] = "value"
	}
	written, err := WriteExportedKey(ctx, client, config.ExportedKey{Key: "target", Type: "hash", Hash: fields, TTL: -1}, true)
	mini.Server().SetPreHook(nil)
	if written || err == nil || calls != 2 {
		t.Fatalf("write=%v err=%v calls=%d", written, err, calls)
	}
	if client.Get(ctx, "target").Val() != "original" || len(client.Keys(ctx, "*").Val()) != 1 {
		t.Fatal("partial construction modified target or leaked stage")
	}
}

func TestStagedImportLostResponseDoesNotRepeat(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	calls := 0
	client.AddHook(lostImportResponseHook{calls: &calls})
	written, err := WriteExportedKey(ctx, client, config.ExportedKey{Key: "target", Type: "list", List: []string{"one"}, TTL: -1}, true)
	if written || RedisErrorCode(err) != "outcome_unknown" || calls != 1 {
		t.Fatalf("response loss=%v %v calls=%d", written, err, calls)
	}
	if client.LLen(ctx, "target").Val() != 1 {
		t.Fatal("write not executed exactly once")
	}
}

type lostImportResponseHook struct{ calls *int }

func (h lostImportResponseHook) DialHook(next goredis.DialHook) goredis.DialHook { return next }
func (h lostImportResponseHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}
func (h lostImportResponseHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		err := next(ctx, cmd)
		if cmd.Name() == "eval" && err == nil {
			(*h.calls)++
			return errors.New("EOF after execution")
		}
		return err
	}
}

func TestStagingKeysStayInTargetSlot(t *testing.T) {
	for _, key := range []string{"normal", "prefix:{tag}:key", "{}empty", "a{unclosed", "a{{nested}x", "a\x00\xff"} {
		temporary, err := stagingKey(key)
		if err != nil || temporary == key || redisKeySlot(temporary) != redisKeySlot(key) {
			t.Fatalf("bad staging slot for %q: %q %v", key, temporary, err)
		}
	}
}

func TestStagedImportRejectsOversizeAndCancelledBeforeSending(t *testing.T) {
	client := testRedisClient(t)
	client.AddHook(exportCommandHook{before: func(cmd goredis.Cmder) error {
		if cmd.Name() == "eval" {
			t.Fatal("invalid or cancelled import sent to Redis")
		}
		return nil
	}})
	for _, entry := range []config.ExportedKey{
		{Key: "large", Type: "string", String: strings.Repeat("v", ImportMaxKeyBytes), TTL: -1},
		{Key: "many", Type: "list", List: make([]string, ImportMaxKeyItems+1), TTL: -1},
	} {
		_, err := WriteExportedKey(context.Background(), client, entry, true)
		if RedisErrorCode(err) != "invalid" {
			t.Fatalf("oversize accepted: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := WriteExportedKey(ctx, client, config.ExportedKey{Key: "target", Type: "string", String: "draft", TTL: -1}, true)
	if RedisErrorCode(err) != "invalid" {
		t.Fatalf("pre-send cancellation=%v", err)
	}
}
