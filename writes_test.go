package main

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"

	"LiteRedis/backend/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func connectedWriteTestApp(t *testing.T) (*App, *miniredis.Miniredis) {
	t.Helper()
	mini := miniredis.RunT(t)
	host, portText, _ := net.SplitHostPort(mini.Addr())
	port, _ := strconv.Atoi(portText)
	a := NewApp()
	a.ctx = context.Background()
	if err := a.manager.Connect(config.ConnectionConfig{ID: "test", Host: host, Port: port, DB: 4}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.manager.DisconnectAll)
	return a, mini
}

func TestRunWritePinsDBAndDoesNotReplayLostResponse(t *testing.T) {
	a, mini := connectedWriteTestApp(t)
	calls := 0
	result := a.runWrite("test", func(ctx context.Context, client redis.UniversalClient) error {
		opts := client.(*redis.Client).Options()
		if opts.DB != 4 || opts.MaxRetries != 0 {
			t.Fatal("write client retries or wrong DB")
		}
		if err := a.manager.SelectDB("test", 7); err != nil {
			t.Fatal(err)
		}
		client.AddHook(writeResponseLossHook{calls: &calls})
		return client.RPush(ctx, "list", "once").Err()
	})
	if result.Success || result.Code != "outcome_unknown" || calls != 1 {
		t.Fatalf("result=%+v calls=%d", result, calls)
	}
	if n, err := mini.DB(4).List("list"); err != nil || len(n) != 1 {
		t.Fatal("write replayed or wrong DB")
	}
	if mini.DB(7).Exists("list") {
		t.Fatal("write followed workspace DB switch")
	}
}

type writeResponseLossHook struct{ calls *int }

func (h writeResponseLossHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h writeResponseLossHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h writeResponseLossHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		if cmd.Name() == "rpush" && err == nil {
			(*h.calls)++
			return errors.New("response lost after write")
		}
		return err
	}
}

func TestCRUDWriteEndpointsUsePinnedDB(t *testing.T) {
	a, mini := connectedWriteTestApp(t)
	for _, result := range []config.OperationResult{
		a.LPush("test", "list", "first"), a.RPush("test", "list", "second"), a.LSet("test", "list", 0, "updated"),
		a.SAdd("test", "set", "one"), a.RenameSetMember("test", "set", "one", "two"), a.SRem("test", "set", "two"),
		a.ZAdd("test", "zset", "one", 1), a.RenameZSetMember("test", "zset", "one", "two"), a.ZRem("test", "zset", "two"),
		a.HSet("test", "hash", "field", "value"), a.HDel("test", "hash", "field"),
		a.CreateKey("test", config.CreateKeyRequest{Key: "new", Type: "string", StringValue: "value", TTL: -1}),
		a.SetTTL("test", "new", 30), a.RenameKey("test", "new", "renamed"), a.DeleteKey("test", "renamed"),
	} {
		if !result.Success {
			t.Fatalf("CRUD failed: %+v", result)
		}
	}
	if values, err := mini.DB(4).List("list"); err != nil || len(values) != 2 || values[0] != "updated" {
		t.Fatalf("list=%v %v", values, err)
	}
	if mini.DB(0).Exists("list") {
		t.Fatal("write used default DB")
	}
}

func TestCreateKeyLocalValidationIsNotUnknown(t *testing.T) {
	a, _ := connectedWriteTestApp(t)
	for _, req := range []config.CreateKeyRequest{{Type: "string"}, {Key: "k", Type: "hash"}, {Key: "k", Type: "unsupported"}} {
		if result := a.CreateKey("test", req); result.Success || result.Code != "invalid" {
			t.Fatalf("local validation=%+v", result)
		}
	}
}
