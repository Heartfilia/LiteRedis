package redis

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"LiteRedis/backend/config"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestGetValuePagesStreamWithoutDuplicates(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	for i := 1; i <= 5; i++ {
		if err := client.XAdd(ctx, &goredis.XAddArgs{
			Stream: "events",
			ID:     fmt.Sprintf("%d-0", i),
			Values: map[string]any{"value": i},
		}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	settings := config.DefaultSettings()
	settings.StreamLoadCount = 2

	var (
		start string
		ids   []string
	)
	for pageNumber := 0; pageNumber < 3; pageNumber++ {
		page, err := GetValue(ctx, client, "events", settings, 0, 0, "", start)
		if err != nil {
			t.Fatalf("page %d: %v", pageNumber+1, err)
		}
		if page.TotalCount != 5 {
			t.Fatalf("page %d total = %d, want 5", pageNumber+1, page.TotalCount)
		}
		for _, entry := range page.StreamVal {
			ids = append(ids, entry.ID)
		}
		if pageNumber < 2 && (!page.HasMore || page.NextStreamID == "") {
			t.Fatalf("page %d has_more=%v next=%q", pageNumber+1, page.HasMore, page.NextStreamID)
		}
		start = page.NextStreamID
	}

	want := []string{"5-0", "4-0", "3-0", "2-0", "1-0"}
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Fatalf("stream IDs = %v, want %v", ids, want)
	}
}

func TestSetStringPreserveTTL(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	if err := client.Set(ctx, "temporary", "old", 30*time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	before, err := client.PTTL(ctx, "temporary").Result()
	if err != nil {
		t.Fatal(err)
	}
	if err := SetStringPreserveTTL(ctx, client, "temporary", "new"); err != nil {
		t.Fatal(err)
	}
	after, err := client.PTTL(ctx, "temporary").Result()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("TTL changed from %s to %s", before, after)
	}
	value, err := client.Get(ctx, "temporary").Result()
	if err != nil || value != "new" {
		t.Fatalf("value = %q, err=%v", value, err)
	}
}

func TestSetStringPreserveTTLKeepsPermanentAndRejectsMissing(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	if err := client.Set(ctx, "permanent", "old", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := SetStringPreserveTTL(ctx, client, "permanent", "new"); err != nil {
		t.Fatal(err)
	}
	ttl, err := client.TTL(ctx, "permanent").Result()
	if err != nil || ttl != -1 {
		t.Fatalf("permanent TTL = %s, err=%v", ttl, err)
	}

	err = SetStringPreserveTTL(ctx, client, "missing", "must-not-create")
	if !errors.Is(err, errStringKeyNotFound) {
		t.Fatalf("expected missing key error, got %v", err)
	}
	if exists, err := client.Exists(ctx, "missing").Result(); err != nil || exists != 0 {
		t.Fatalf("missing key was recreated: exists=%d err=%v", exists, err)
	}
}

func testRedisClient(t *testing.T) *goredis.Client {
	t.Helper()
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func TestRenameSetMember(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)

	if err := client.SAdd(ctx, "members", "source", "destination").Err(); err != nil {
		t.Fatal(err)
	}
	if err := RenameSetMember(ctx, client, "members", "source", "destination"); err != nil {
		t.Fatalf("rename set member: %v", err)
	}

	members, err := client.SMembers(ctx, "members").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0] != "destination" {
		t.Fatalf("unexpected members after rename: %v", members)
	}
}

func TestRenameSetMemberMissingSourceDoesNotMutate(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)

	if err := client.SAdd(ctx, "members", "untouched").Err(); err != nil {
		t.Fatal(err)
	}
	err := RenameSetMember(ctx, client, "members", "missing", "destination")
	if !errors.Is(err, errSetMemberNotFound) {
		t.Fatalf("expected missing member error, got %v", err)
	}

	members, err := client.SMembers(ctx, "members").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0] != "untouched" {
		t.Fatalf("set changed after failed rename: %v", members)
	}
}

func TestRenameZSetMemberPreservesCurrentScore(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)

	if err := client.ZAdd(ctx, "rank", goredis.Z{Member: "source", Score: 42.5}, goredis.Z{Member: "destination", Score: 1}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := RenameZSetMember(ctx, client, "rank", "source", "destination"); err != nil {
		t.Fatalf("rename zset member: %v", err)
	}

	score, err := client.ZScore(ctx, "rank", "destination").Result()
	if err != nil {
		t.Fatal(err)
	}
	if score != 42.5 {
		t.Fatalf("destination score = %v, want 42.5", score)
	}
	if exists, err := client.ZScore(ctx, "rank", "source").Result(); err != goredis.Nil || exists != 0 {
		t.Fatalf("source still exists: score=%v err=%v", exists, err)
	}
}

func TestRenameZSetMemberMissingSourceDoesNotMutate(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)

	if err := client.ZAdd(ctx, "rank", goredis.Z{Member: "untouched", Score: 7}).Err(); err != nil {
		t.Fatal(err)
	}
	err := RenameZSetMember(ctx, client, "rank", "missing", "destination")
	if !errors.Is(err, errZSetMemberNotFound) {
		t.Fatalf("expected missing member error, got %v", err)
	}

	score, err := client.ZScore(ctx, "rank", "untouched").Result()
	if err != nil || score != 7 {
		t.Fatalf("zset changed after failed rename: score=%v err=%v", score, err)
	}
}
