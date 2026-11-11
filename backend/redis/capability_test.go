package redis

import (
	"context"
	"errors"
	goredis "github.com/redis/go-redis/v9"
	"testing"
)

func TestMemoryUnavailableReasons(t *testing.T) {
	for _, tc := range []struct{ message, reason string }{
		{"ERR unknown command 'MEMORY'", "unsupported_or_renamed"},
		{"NOPERM MEMORY denied", "permission_denied"},
		{"ERR command disabled", "command_disabled"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			client := testRedisClient(t)
			client.AddHook(memoryReportHook{process: func(cmd goredis.Cmder) bool {
				switch cmd.Name() {
				case "memory":
					cmd.SetErr(errors.New(tc.message))
					return true
				case "info":
					cmd.(*goredis.StringCmd).SetVal("used_memory:10000\r\n")
					return true
				}
				return false
			}})
			report, err := ScanMemory(context.Background(), client, "*", 100, nil)
			if err != nil || report.MemorySharePercent != nil || report.MemoryShareUnavailableReason != "memory_usage_"+tc.reason {
				t.Fatalf("report=%+v err=%v", report, err)
			}
		})
	}
}
