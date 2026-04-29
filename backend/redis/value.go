package redis

import (
	"context"
	"fmt"
	"time"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

// GetValue 读取 key 的值（按类型分支），支持 cursor/offset 分页。
// cursor=0, offset=0 表示第一页。loadCount 控制每次加载条数（≤0 取默认）。
func GetValue(ctx context.Context, client redis.UniversalClient, key string, settings config.AppSettings, cursor uint64, offset int, zsetSort string) (config.KeyValue, error) {
	keyInfo, err := GetKeyInfo(ctx, client, key)
	if err != nil {
		return config.KeyValue{}, err
	}

	kv := config.KeyValue{
		Key:        key,
		Type:       keyInfo.Type,
		TTL:        keyInfo.TTL,
		TotalCount: -1,
	}

	def := config.DefaultSettings()

	hashCount := settings.HashLoadCount
	if hashCount <= 0 {
		hashCount = def.HashLoadCount
	}
	listCount := settings.ListLoadCount
	if listCount <= 0 {
		listCount = def.ListLoadCount
	}
	setCount := settings.SetLoadCount
	if setCount <= 0 {
		setCount = def.SetLoadCount
	}
	zsetCount := settings.ZSetLoadCount
	if zsetCount <= 0 {
		zsetCount = def.ZSetLoadCount
	}
	streamCount := settings.StreamLoadCount
	if streamCount <= 0 {
		streamCount = def.StreamLoadCount
	}

	switch keyInfo.Type {
	case "string":
		val, err := client.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return kv, err
		}
		kv.StringVal = val
		kv.HasMore = false

	case "hash":
		result := make(map[string]string)
		loaded := int64(0)
		for {
			keys, newCursor, err := client.HScan(ctx, key, cursor, "*", hashCount).Result()
			if err != nil {
				return kv, err
			}
			for i := 0; i+1 < len(keys); i += 2 {
				result[keys[i]] = keys[i+1]
				loaded++
			}
			cursor = newCursor
			if cursor == 0 || loaded >= hashCount {
				break
			}
		}
		kv.HashVal = result
		kv.NextCursor = cursor
		kv.HasMore = cursor != 0
		total, _ := client.HLen(ctx, key).Result()
		kv.TotalCount = total

	case "list":
		end := int64(offset) + listCount - 1
		val, err := client.LRange(ctx, key, int64(offset), end).Result()
		if err != nil {
			return kv, err
		}
		total, _ := client.LLen(ctx, key).Result()
		kv.ListVal = val
		kv.NextOffset = offset + len(val)
		kv.TotalCount = total
		kv.HasMore = int64(offset+len(val)) < total

	case "set":
		var members []string
		for {
			batch, newCursor, err := client.SScan(ctx, key, cursor, "*", setCount).Result()
			if err != nil {
				return kv, err
			}
			members = append(members, batch...)
			cursor = newCursor
			if cursor == 0 || int64(len(members)) >= setCount {
				break
			}
		}
		kv.SetVal = members
		kv.NextCursor = cursor
		kv.HasMore = cursor != 0
		total, _ := client.SCard(ctx, key).Result()
		kv.TotalCount = total

	case "zset":
		end := int64(offset) + zsetCount - 1
		var (
			vals []redis.Z
			err  error
		)
		if zsetSort == "desc" {
			vals, err = client.ZRevRangeWithScores(ctx, key, int64(offset), end).Result()
		} else {
			vals, err = client.ZRangeWithScores(ctx, key, int64(offset), end).Result()
		}
		if err != nil {
			return kv, err
		}
		total, _ := client.ZCard(ctx, key).Result()
		members := make([]config.ZSetMember, len(vals))
		for i, z := range vals {
			members[i] = config.ZSetMember{
				Member: fmt.Sprintf("%v", z.Member),
				Score:  z.Score,
			}
		}
		kv.ZSetVal = members
		kv.NextOffset = offset + len(vals)
		kv.TotalCount = total
		kv.HasMore = int64(offset+len(vals)) < total

	case "stream":
		vals, err := client.XRevRangeN(ctx, key, "+", "-", streamCount).Result()
		if err != nil {
			return kv, err
		}
		entries := make([]config.StreamEntry, len(vals))
		for i, msg := range vals {
			fields := make(map[string]string, len(msg.Values))
			for k, v := range msg.Values {
				fields[k] = fmt.Sprintf("%v", v)
			}
			entries[i] = config.StreamEntry{
				ID:     msg.ID,
				Fields: fields,
			}
		}
		kv.StreamVal = entries
		kv.HasMore = false
	}

	return kv, nil
}

// SetString 设置 string 类型
func SetString(ctx context.Context, client redis.UniversalClient, key, value string, ttlSec int64) error {
	var ttl time.Duration
	if ttlSec > 0 {
		ttl = time.Duration(ttlSec) * time.Second
	}
	return client.Set(ctx, key, value, ttl).Err()
}

// HSet 设置 hash field
func HSet(ctx context.Context, client redis.UniversalClient, key, field, value string) error {
	return client.HSet(ctx, key, field, value).Err()
}

// HDel 删除 hash field
func HDel(ctx context.Context, client redis.UniversalClient, key, field string) error {
	return client.HDel(ctx, key, field).Err()
}

// LPush 向 list 头部插入
func LPush(ctx context.Context, client redis.UniversalClient, key, value string) error {
	return client.LPush(ctx, key, value).Err()
}

// RPush 向 list 尾部插入
func RPush(ctx context.Context, client redis.UniversalClient, key, value string) error {
	return client.RPush(ctx, key, value).Err()
}

// LSet 设置 list 指定索引的值
func LSet(ctx context.Context, client redis.UniversalClient, key string, index int64, value string) error {
	return client.LSet(ctx, key, index, value).Err()
}

// LRem 删除 list 中与 value 相等的元素（count=0 全部删除）
func LRem(ctx context.Context, client redis.UniversalClient, key string, count int64, value string) error {
	return client.LRem(ctx, key, count, value).Err()
}

// SAdd 向 set 添加成员
func SAdd(ctx context.Context, client redis.UniversalClient, key, member string) error {
	return client.SAdd(ctx, key, member).Err()
}

// SRem 从 set 删除成员
func SRem(ctx context.Context, client redis.UniversalClient, key, member string) error {
	return client.SRem(ctx, key, member).Err()
}

// ZAdd 向 zset 添加成员
func ZAdd(ctx context.Context, client redis.UniversalClient, key, member string, score float64) error {
	return client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// ZRem 从 zset 删除成员
func ZRem(ctx context.Context, client redis.UniversalClient, key, member string) error {
	return client.ZRem(ctx, key, member).Err()
}
