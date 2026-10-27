package redis

import (
	"context"
	"fmt"
	"testing"

	"LiteRedis/backend/config"
)

func TestListChunkPagingMatchesFullList(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	InvalidateListChunks("page|0", "biglist")

	for i := 0; i < 620; i++ {
		if err := client.RPush(ctx, "biglist", fmt.Sprintf("v%d", i)).Err(); err != nil {
			t.Fatal(err)
		}
	}

	settings := config.DefaultSettings()
	settings.ListLoadCount = 20

	var got []string
	for offset := 0; offset < 620; offset += 20 {
		page, err := GetValue(ctx, client, "biglist", settings, 0, offset, "", "", "page|0")
		if err != nil {
			t.Fatalf("offset %d: %v", offset, err)
		}
		got = append(got, page.ListVal...)
		if page.TotalCount != 620 {
			t.Fatalf("total = %d", page.TotalCount)
		}
	}
	if len(got) != 620 {
		t.Fatalf("collected %d items", len(got))
	}
	for i, value := range got {
		if value != fmt.Sprintf("v%d", i) {
			t.Fatalf("item %d = %q", i, value)
		}
	}
}

func TestListChunkCacheInvalidation(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	InvalidateListChunks("inv|0", "list")

	for i := 0; i < 100; i++ {
		if err := client.RPush(ctx, "list", fmt.Sprintf("v%d", i)).Err(); err != nil {
			t.Fatal(err)
		}
	}
	settings := config.DefaultSettings()
	settings.ListLoadCount = 20

	if _, err := GetValue(ctx, client, "list", settings, 0, 0, "", "", "inv|0"); err != nil {
		t.Fatal(err)
	}
	page, err := GetValue(ctx, client, "list", settings, 0, 20, "", "", "inv|0")
	if err != nil {
		t.Fatal(err)
	}
	if page.ListVal[5] != "v25" {
		t.Fatalf("offset 20 index 5 = %q", page.ListVal[5])
	}

	if err := client.LSet(ctx, "list", 25, "changed").Err(); err != nil {
		t.Fatal(err)
	}

	// 未失效时命中缓存，仍是旧值
	cached, err := GetValue(ctx, client, "list", settings, 0, 20, "", "", "inv|0")
	if err != nil {
		t.Fatal(err)
	}
	if cached.ListVal[5] != "v25" {
		t.Fatalf("cache should still serve old value, got %q", cached.ListVal[5])
	}

	InvalidateListChunks("inv|0", "list")
	fresh, err := GetValue(ctx, client, "list", settings, 0, 20, "", "", "inv|0")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ListVal[5] != "changed" {
		t.Fatalf("after invalidation got %q", fresh.ListVal[5])
	}
}
