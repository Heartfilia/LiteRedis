package redis

import (
	"context"
	"testing"

	"LiteRedis/backend/config"

	goredis "github.com/redis/go-redis/v9"
)

func TestGetValueKnownType(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	InvalidateListChunks("fast|0", "listkey")

	if err := client.Set(ctx, "strkey", "hello", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.HSet(ctx, "hashkey", "f1", "v1", "f2", "v2").Err(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		if err := client.RPush(ctx, "listkey", "v").Err(); err != nil {
			t.Fatal(err)
		}
	}
	if err := client.SAdd(ctx, "setkey", "a", "b").Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.ZAdd(ctx, "zsetkey", goredis.Z{Score: 1.5, Member: "m1"}, goredis.Z{Score: 2, Member: "m2"}).Err(); err != nil {
		t.Fatal(err)
	}

	settings := config.DefaultSettings()

	str, err := GetValue(ctx, client, "strkey", settings, 0, 0, "", "", "fast|0", "string")
	if err != nil || str.StringVal != "hello" || str.Type != "string" {
		t.Fatalf("string fast path = %#v, err=%v", str, err)
	}

	hash, err := GetValue(ctx, client, "hashkey", settings, 0, 0, "", "", "fast|0", "hash")
	if err != nil || len(hash.HashVal) != 2 || hash.TotalCount != 2 {
		t.Fatalf("hash fast path = %#v, err=%v", hash, err)
	}

	list, err := GetValue(ctx, client, "listkey", settings, 0, 0, "", "", "fast|0", "list")
	if err != nil || len(list.ListVal) != 20 || list.TotalCount != 30 || !list.HasMore {
		t.Fatalf("list fast path = %#v, err=%v", list, err)
	}

	set, err := GetValue(ctx, client, "setkey", settings, 0, 0, "", "", "fast|0", "set")
	if err != nil || len(set.SetVal) != 2 || set.TotalCount != 2 {
		t.Fatalf("set fast path = %#v, err=%v", set, err)
	}

	zset, err := GetValue(ctx, client, "zsetkey", settings, 0, 0, "", "", "fast|0", "zset")
	if err != nil || len(zset.ZSetVal) != 2 || zset.TotalCount != 2 {
		t.Fatalf("zset fast path = %#v, err=%v", zset, err)
	}
}

func TestGetValueWrongTypeHintFallsBack(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	if err := client.Set(ctx, "onlystring", "value", 0).Err(); err != nil {
		t.Fatal(err)
	}

	settings := config.DefaultSettings()
	// 类型给错时应回退到探测路径，按真实类型返回
	kv, err := GetValue(ctx, client, "onlystring", settings, 0, 0, "", "", "fallback|0", "hash")
	if err != nil {
		t.Fatalf("fallback failed: %v", err)
	}
	if kv.Type != "string" || kv.StringVal != "value" {
		t.Fatalf("fallback result = %#v", kv)
	}
}
