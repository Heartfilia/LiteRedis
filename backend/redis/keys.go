package redis

import (
	"context"
	"time"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

// ScanKeys 扫描 key，通过 Pipeline 批量获取 TYPE 和 TTL
func ScanKeys(ctx context.Context, client redis.UniversalClient, pattern string, count int64) ([]config.RedisKey, error) {
	if pattern == "" {
		pattern = "*"
	}
	if count <= 0 {
		count = 100
	}

	var keyNames []string

	// 使用 SCAN 避免阻塞（不使用 KEYS）
	iter := client.Scan(ctx, 0, pattern, count).Iterator()
	for iter.Next(ctx) {
		keyNames = append(keyNames, iter.Val())
		if int64(len(keyNames)) >= count {
			break
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	if len(keyNames) == 0 {
		return []config.RedisKey{}, nil
	}

	// Pipeline 批量获取 TYPE + TTL
	pipe := client.Pipeline()
	typeCmds := make([]*redis.StatusCmd, len(keyNames))
	ttlCmds := make([]*redis.DurationCmd, len(keyNames))

	for i, k := range keyNames {
		typeCmds[i] = pipe.Type(ctx, k)
		ttlCmds[i] = pipe.TTL(ctx, k)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	keys := make([]config.RedisKey, len(keyNames))
	for i, name := range keyNames {
		ttlDur := ttlCmds[i].Val()
		var ttlSec int64
		if ttlDur < 0 {
			ttlSec = int64(ttlDur) // -1 或 -2
		} else {
			ttlSec = int64(ttlDur / time.Second)
		}
		keys[i] = config.RedisKey{
			Name: name,
			Type: typeCmds[i].Val(),
			TTL:  ttlSec,
		}
	}
	return keys, nil
}

// GetKeyInfo 获取单个 key 的元信息
func GetKeyInfo(ctx context.Context, client redis.UniversalClient, key string) (config.RedisKey, error) {
	pipe := client.Pipeline()
	typeCmd := pipe.Type(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return config.RedisKey{}, err
	}

	ttlDur := ttlCmd.Val()
	var ttlSec int64
	if ttlDur < 0 {
		ttlSec = int64(ttlDur)
	} else {
		ttlSec = int64(ttlDur / time.Second)
	}

	return config.RedisKey{
		Name: key,
		Type: typeCmd.Val(),
		TTL:  ttlSec,
	}, nil
}

// DeleteKey 删除 key
func DeleteKey(ctx context.Context, client redis.UniversalClient, key string) error {
	return client.Del(ctx, key).Err()
}

// RenameKey 重命名 key
func RenameKey(ctx context.Context, client redis.UniversalClient, oldKey, newKey string) error {
	return client.Rename(ctx, oldKey, newKey).Err()
}

// SetTTL 设置 key TTL（-1 表示永久，即 PERSIST）
func SetTTL(ctx context.Context, client redis.UniversalClient, key string, ttlSec int64) error {
	if ttlSec < 0 {
		return client.Persist(ctx, key).Err()
	}
	return client.Expire(ctx, key, time.Duration(ttlSec)*time.Second).Err()
}

// DBSize 获取当前 DB key 总数
func DBSize(ctx context.Context, client redis.UniversalClient) (int64, error) {
	return client.DBSize(ctx).Result()
}
