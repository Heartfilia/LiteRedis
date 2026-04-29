package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

// ScanKeys 扫描 key（支持 cursor 分页），通过 Pipeline 批量获取 TYPE 和 TTL
func ScanKeys(ctx context.Context, client redis.UniversalClient, pattern string, count int64, cursor uint64) (config.ScanResult, error) {
	result := config.ScanResult{Keys: []config.RedisKey{}}
	if pattern == "" {
		pattern = "*"
	}
	if count <= 0 {
		count = 100
	}

	var keyNames []string
	var nextCursor uint64

	// 使用 SCAN 避免阻塞，循环直到获取到数据或 cursor 回到 0
	for {
		keys, newCursor, err := client.Scan(ctx, cursor, pattern, count).Result()
		if err != nil {
			return result, err
		}
		keyNames = append(keyNames, keys...)
		cursor = newCursor
		if cursor == 0 || int64(len(keyNames)) >= count {
			nextCursor = cursor
			break
		}
		// 如果一批返回空但 cursor 不为 0，继续 scan
		if len(keys) == 0 {
			continue
		}
	}

	if len(keyNames) == 0 {
		return result, nil
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
		return result, err
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

	result.Keys = keys
	result.NextCursor = nextCursor
	result.HasMore = nextCursor != 0
	return result, nil
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

// CreateKey 创建指定类型的 key，并按需设置初始值和 TTL。
func CreateKey(ctx context.Context, client redis.UniversalClient, req config.CreateKeyRequest) error {
	key := strings.TrimSpace(req.Key)
	keyType := strings.ToLower(strings.TrimSpace(req.Type))
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if keyType == "" {
		return fmt.Errorf("type is required")
	}

	exists, err := client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return fmt.Errorf("key already exists")
	}

	switch keyType {
	case "string":
		if err := client.Set(ctx, key, req.StringValue, 0).Err(); err != nil {
			return err
		}
	case "hash":
		field := strings.TrimSpace(req.Field)
		if field == "" {
			return fmt.Errorf("field is required for hash")
		}
		if err := client.HSet(ctx, key, field, req.Value).Err(); err != nil {
			return err
		}
	case "list":
		if err := client.RPush(ctx, key, req.ListValue).Err(); err != nil {
			return err
		}
	case "set":
		member := strings.TrimSpace(req.Member)
		if member == "" {
			return fmt.Errorf("member is required for set")
		}
		if err := client.SAdd(ctx, key, member).Err(); err != nil {
			return err
		}
	case "zset":
		member := strings.TrimSpace(req.Member)
		if member == "" {
			return fmt.Errorf("member is required for zset")
		}
		if err := client.ZAdd(ctx, key, redis.Z{Score: req.Score, Member: member}).Err(); err != nil {
			return err
		}
	case "stream":
		field := strings.TrimSpace(req.Field)
		if field == "" {
			return fmt.Errorf("field is required for stream")
		}
		if _, err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: key,
			Values: map[string]interface{}{field: req.Value},
		}).Result(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported type: %s", keyType)
	}

	return SetTTL(ctx, client, key, req.TTL)
}

// DBSize 获取当前 DB key 总数
func DBSize(ctx context.Context, client redis.UniversalClient) (int64, error) {
	return client.DBSize(ctx).Result()
}
