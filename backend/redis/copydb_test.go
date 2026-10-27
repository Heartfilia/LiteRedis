package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestCopyKeysToDB(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)

	db0 := goredis.NewClient(&goredis.Options{Addr: server.Addr(), DB: 0})
	db2 := goredis.NewClient(&goredis.Options{Addr: server.Addr(), DB: 2})
	defer db0.Close()
	defer db2.Close()

	if err := db0.Set(ctx, "user:1", "alice", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := db0.Set(ctx, "existing", "keep", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := db2.Set(ctx, "existing", "old", 0).Err(); err != nil {
		t.Fatal(err)
	}

	copied, skipped, failed, err := CopyKeysToDB(ctx, db0, []string{"user:1", "existing"}, 2, false)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if copied != 1 || skipped != 1 || failed != 0 {
		t.Fatalf("copied=%d skipped=%d failed=%d", copied, skipped, failed)
	}
	if value, err := db2.Get(ctx, "user:1").Result(); err != nil || value != "alice" {
		t.Fatalf("db2 value = %q, err=%v", value, err)
	}
	if value, _ := db2.Get(ctx, "existing").Result(); value != "old" {
		t.Fatalf("existing key was overwritten: %q", value)
	}

	// 覆盖模式
	copied, _, failed, err = CopyKeysToDB(ctx, db0, []string{"existing"}, 2, true)
	if err != nil || copied != 1 || failed != 0 {
		t.Fatalf("overwrite copy: copied=%d failed=%d err=%v", copied, failed, err)
	}
	if value, _ := db2.Get(ctx, "existing").Result(); value != "keep" {
		t.Fatalf("overwrite did not take effect: %q", value)
	}
}
