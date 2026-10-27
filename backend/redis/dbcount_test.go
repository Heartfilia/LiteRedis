package redis

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestGetDatabaseCountFallbacks(t *testing.T) {
	ctx := context.Background()

	// 连不上的实例：CONFIG GET 失败，回退 16
	broken := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 200 * time.Millisecond})
	defer broken.Close()
	if got := GetDatabaseCount(ctx, broken); got != 16 {
		t.Fatalf("broken client count = %d, want fallback 16", got)
	}

	// 集群没有 DB 概念
	cluster := goredis.NewClusterClient(&goredis.ClusterOptions{Addrs: []string{"127.0.0.1:1"}})
	defer cluster.Close()
	if got := GetDatabaseCount(ctx, cluster); got != 1 {
		t.Fatalf("cluster count = %d, want 1", got)
	}
}
