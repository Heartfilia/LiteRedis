package redis

import (
	"context"
	"errors"
	goredis "github.com/redis/go-redis/v9"
	"testing"
	"time"
)

func TestCheckedStringPreservesTTLAndRejectsConflicts(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	client.Set(ctx, "s", "original", time.Minute)
	preview, err := readStringPreview(ctx, client, "s")
	if err != nil {
		t.Fatal(err)
	}
	client.Set(ctx, "s", "business update", time.Minute)
	if result := SetStringChecked(ctx, client, "s", "draft", preview.EditToken); result.Code != "conflict" {
		t.Fatalf("conflict overwritten: %+v", result)
	}
	if client.Get(ctx, "s").Val() != "business update" {
		t.Fatal("lost external update")
	}
	preview, _ = readStringPreview(ctx, client, "s")
	result := SetStringChecked(ctx, client, "s", "draft", preview.EditToken)
	if !result.Success || result.EditToken != stringEditToken("draft") || client.PTTL(ctx, "s").Val() <= 0 {
		t.Fatalf("save failed or TTL lost: %+v", result)
	}
	client.Del(ctx, "s")
	if result := SetStringChecked(ctx, client, "s", "resurrect", result.EditToken); result.Code != "conflict" {
		t.Fatal("expired key recreated")
	}
}

func TestCheckedEditsDoNotFallbackOrReplayUnknownOutcomes(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{errors.New("NOPERM EVAL denied"), "permission_denied"},
		{errors.New("ERR ACL failure in script: User limited has no permissions to run the 'expire' command"), "partial_possible"},
		{errors.New("ERR unknown command 'EVAL'"), "unsupported"},
		{errors.New("MOVED 1234 127.0.0.1:7000"), "cluster_redirect"},
		{errors.New("connection reset after write"), "outcome_unknown"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			client := testRedisClient(t)
			calls := 0
			client.AddHook(memoryReportHook{process: func(cmd goredis.Cmder) bool {
				calls++
				if cmd.Name() != "eval" {
					t.Fatalf("unprotected fallback command: %s", cmd.Name())
				}
				cmd.SetErr(tc.err)
				return true
			}})
			result := SetStringChecked(context.Background(), client, "key", "draft", stringEditToken("old"))
			if calls != 1 || result.Success || result.Code != tc.code {
				t.Fatalf("unsafe save result=%+v calls=%d", result, calls)
			}
		})
	}
}

func TestCheckedHashFieldDetectsMissingAndConcurrentChanges(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	client.HSet(ctx, "h", "f", "old", "other", "one")
	client.Expire(ctx, "h", time.Minute)
	if result := HSetChecked(ctx, client, "h", "f", "new", "stale", true); result.Code != "conflict" {
		t.Fatalf("stale field saved: %+v", result)
	}
	if result := HSetChecked(ctx, client, "h", "f", "new", "", false); result.Code != "conflict" {
		t.Fatal("existing field overwritten by add")
	}
	if result := HSetChecked(ctx, client, "h", "f", "new", "old", true); !result.Success {
		t.Fatalf("valid field save: %+v", result)
	}
	if result := HSetChecked(ctx, client, "h", "empty", "", "", false); !result.Success {
		t.Fatal("empty field value rejected")
	}
	if client.PTTL(ctx, "h").Val() <= 0 {
		t.Fatal("hash TTL lost")
	}
	client.Del(ctx, "h")
	if result := HSetChecked(ctx, client, "h", "f", "new", "new", true); result.Code != "conflict" {
		t.Fatal("missing hash recreated")
	}
}
