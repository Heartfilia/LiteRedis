package redis

import (
	"context"
	"errors"
	goredis "github.com/redis/go-redis/v9"
	"testing"
	"time"
)

func TestTTLBatchCountsConfirmedSuccessBeforeFailure(t *testing.T) {
	client := testRedisClient(t)
	client.AddHook(memoryReportHook{process: func(cmd goredis.Cmder) bool {
		if cmd.Name() != "expire" {
			return false
		}
		if cmd.Args()[1] == "ok" {
			cmd.(*goredis.BoolCmd).SetVal(true)
		} else {
			cmd.SetErr(errors.New("NOPERM EXPIRE denied"))
		}
		return true
	}})
	affected, err := SetKeysTTL(context.Background(), client, []string{"ok", "bad"}, 60)
	if affected != 1 || err == nil {
		t.Fatalf("confirmed=%d err=%v", affected, err)
	}
}

func TestTTLOverflowNeverDeletesKey(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	client.Set(ctx, "target", "original", time.Hour)
	if err := SetTTL(ctx, client, "target", 1<<62); err == nil {
		t.Fatal("overflow TTL accepted")
	}
	if client.Get(ctx, "target").Val() != "original" {
		t.Fatal("TTL overflow deleted original")
	}
}

func TestScriptRuntimeErrorMayHavePartiallyExecuted(t *testing.T) {
	if code := RedisErrorCode(errors.New("ERR Error running script: NOPERM @user_script:12")); code != "partial_possible" {
		t.Fatalf("script partial execution hidden: %s", code)
	}
}
