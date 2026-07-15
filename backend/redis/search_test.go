package redis

import (
	"context"
	"fmt"
	"testing"

	"LiteRedis/backend/config"

	goredis "github.com/redis/go-redis/v9"
)

func TestSearchListPaginatesWithoutDroppingMatches(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	values := []interface{}{"skip", "match-one", "skip", "match-two", "match-three"}
	if err := client.RPush(ctx, "items", values...).Err(); err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultSettings()
	settings.ListLoadCount = 2

	first, err := SearchValue(ctx, client, "items", "list", "match", settings, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(first.ListVal) != "[match-one match-two]" || !first.HasMore || first.NextCursor != 4 {
		t.Fatalf("unexpected first page: values=%v hasMore=%v cursor=%d", first.ListVal, first.HasMore, first.NextCursor)
	}

	second, err := SearchValue(ctx, client, "items", "list", "match", settings, false, first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(second.ListVal) != "[match-three]" || second.HasMore || second.NextCursor != 0 {
		t.Fatalf("unexpected second page: values=%v hasMore=%v cursor=%d", second.ListVal, second.HasMore, second.NextCursor)
	}
}

func TestSearchZSetReturnsEveryScannedMemberAcrossPages(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	for i := 0; i < 25; i++ {
		member := fmt.Sprintf("member:%02d", i)
		if err := client.ZAdd(ctx, "rank", goredis.Z{Member: member, Score: float64(i)}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	settings := config.DefaultSettings()
	settings.ZSetLoadCount = 3

	seen := make(map[string]struct{})
	var cursor uint64
	for page := 0; page < 50; page++ {
		result, err := SearchValue(ctx, client, "rank", "zset", "member:*", settings, false, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, member := range result.ZSetVal {
			seen[member.Member] = struct{}{}
		}
		if !result.HasMore {
			break
		}
		if result.NextCursor == 0 {
			t.Fatal("zset search reported more data without a continuation cursor")
		}
		cursor = result.NextCursor
	}
	if len(seen) != 25 {
		t.Fatalf("zset search returned %d unique members, want 25", len(seen))
	}
}
