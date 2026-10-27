package redis

import (
	"testing"
	"time"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

func TestSortSlowLogByDurationThenTime(t *testing.T) {
	entries := []config.SlowLogEntry{
		{DurationUs: 10, Timestamp: 1},
		{DurationUs: 100, Timestamp: 2},
		{DurationUs: 10, Timestamp: 5},
	}
	sortSlowLog(entries)
	if entries[0].DurationUs != 100 {
		t.Fatalf("slowest entry not first: %#v", entries)
	}
	if entries[1].Timestamp != 5 {
		t.Fatalf("tie should prefer newer entry: %#v", entries)
	}
}

func TestSlowLogEntryConversion(t *testing.T) {
	item := redis.SlowLog{
		ID:         7,
		Time:       time.Unix(1_700_000_000, 0),
		Duration:   3 * time.Millisecond,
		Args:       []string{"HGETALL", "cart:1"},
		ClientAddr: "10.0.0.9:51234",
		ClientName: "app",
	}
	entry := slowLogEntry(item, "10.0.0.1:6379")
	if entry.ID != 7 || entry.Timestamp != 1_700_000_000 {
		t.Fatalf("id/timestamp wrong: %#v", entry)
	}
	if entry.DurationUs != 3000 {
		t.Fatalf("duration = %d us", entry.DurationUs)
	}
	if entry.Command != "HGETALL cart:1" {
		t.Fatalf("command = %q", entry.Command)
	}
	if entry.Node != "10.0.0.1:6379" || entry.ClientAddr != "10.0.0.9:51234" {
		t.Fatalf("node/client wrong: %#v", entry)
	}
}
