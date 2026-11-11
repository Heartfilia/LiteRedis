package redis

import (
	"context"
	"errors"
	goredis "github.com/redis/go-redis/v9"
	"testing"
)

func TestConsoleRejectsSessionStateAndHandlesNil(t *testing.T) {
	client := testRedisClient(t)
	for _, command := range []string{"MULTI", "EXEC", "WATCH key", "SUBSCRIBE channel", "AUTH password", "CLIENT REPLY OFF", "MONITOR", "HELLO 2"} {
		result := ExecuteRedisCommandWithState(context.Background(), client, command, "", nil)
		if result.Success || result.Code != "rejected" {
			t.Fatalf("session command allowed: %q %+v", command, result)
		}
	}
	if result := ExecuteRedisCommand(context.Background(), client, "GET missing"); !result.Success || result.Output != "(nil)" {
		t.Fatalf("nil GET: %+v", result)
	}
}

func TestConsoleDistinguishesReadFailureAndUnknownWrite(t *testing.T) {
	client := testRedisClient(t)
	client.AddHook(memoryReportHook{process: func(cmd goredis.Cmder) bool { cmd.SetErr(errors.New("response lost")); return true }})
	for _, tc := range []struct{ command, code string }{{"GET key", "read_failed"}, {"INCR key", "outcome_unknown"}, {"XADD stream * f v", "outcome_unknown"}} {
		if result := ExecuteRedisCommand(context.Background(), client, tc.command); result.Code != tc.code {
			t.Fatalf("%q: %+v", tc.command, result)
		}
	}
}

type countingConsoleState struct{ calls int }

func (s *countingConsoleState) SelectDB(string, int) error { s.calls++; return nil }

func TestConsoleSelectCannotBypassDraftGuardSyntax(t *testing.T) {
	client := testRedisClient(t)
	state := &countingConsoleState{}
	for _, command := range []string{`"SELECT" 2`, `SELECT "2"`, `SE\LECT 2`, `SELECT +2`, `SELECT 2 extra`} {
		if result := ExecuteRedisCommandWithState(context.Background(), client, command, "conn", state); result.Success {
			t.Fatalf("unguarded SELECT accepted: %s", command)
		}
	}
	if state.calls != 0 {
		t.Fatal("invalid SELECT changed workspace DB")
	}
	if result := ExecuteRedisCommandWithState(context.Background(), client, "SELECT 2", "conn", state); !result.Success || state.calls != 1 {
		t.Fatal("valid SELECT failed")
	}
}
