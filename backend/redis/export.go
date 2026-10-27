package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

const (
	// ExportMaxKeys 单次导出/复制最多处理的 key 数
	ExportMaxKeys = 5000
	// ExportMaxStringBytes 单个 string 值的上限，超过直接报错而不是截断
	ExportMaxStringBytes = 32 << 20
)

// ReadKeyForExport 按类型把 key 的完整内容读出来
func ReadKeyForExport(ctx context.Context, client redis.UniversalClient, key string) (config.ExportedKey, error) {
	info, err := GetKeyInfo(ctx, client, key)
	if err != nil {
		return config.ExportedKey{}, err
	}
	entry := config.ExportedKey{Key: key, Type: info.Type, TTL: info.TTL}

	switch info.Type {
	case "string":
		value, err := client.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return entry, err
		}
		if len(value) > ExportMaxStringBytes {
			return entry, fmt.Errorf("key %s 的字符串超过 32MB，已跳过", key)
		}
		entry.String = value
	case "hash":
		values, err := client.HGetAll(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return entry, err
		}
		entry.Hash = values
	case "list":
		values, err := client.LRange(ctx, key, 0, -1).Result()
		if err != nil && err != redis.Nil {
			return entry, err
		}
		entry.List = values
	case "set":
		values, err := client.SMembers(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return entry, err
		}
		sort.Strings(values)
		entry.Set = values
	case "zset":
		values, err := client.ZRangeWithScores(ctx, key, 0, -1).Result()
		if err != nil && err != redis.Nil {
			return entry, err
		}
		members := make([]config.ZSetMember, len(values))
		for i, item := range values {
			members[i] = config.ZSetMember{Member: item.Member.(string), Score: item.Score}
		}
		entry.ZSet = members
	case "stream":
		values, err := client.XRange(ctx, key, "-", "+").Result()
		if err != nil && err != redis.Nil {
			return entry, err
		}
		entries := make([]config.StreamEntry, len(values))
		for i, item := range values {
			fields := make(map[string]string, len(item.Values))
			for field, raw := range item.Values {
				fields[field] = fmt.Sprint(raw)
			}
			entries[i] = config.StreamEntry{ID: item.ID, Fields: fields}
		}
		entry.Stream = entries
	default:
		return entry, fmt.Errorf("不支持导出的类型: %s", info.Type)
	}
	return entry, nil
}

// WriteExportedKey 把导出的 key 写回（用于跨实例复制）。
// overwrite=false 时目标已存在则跳过，返回 false。
func WriteExportedKey(ctx context.Context, client redis.UniversalClient, entry config.ExportedKey, overwrite bool) (bool, error) {
	if entry.Key == "" {
		return false, nil
	}
	exists, err := client.Exists(ctx, entry.Key).Result()
	if err != nil {
		return false, err
	}
	if exists > 0 {
		if !overwrite {
			return false, nil
		}
		if err := client.Del(ctx, entry.Key).Err(); err != nil {
			return false, err
		}
	}

	pipe := client.Pipeline()
	switch entry.Type {
	case "string":
		pipe.Set(ctx, entry.Key, entry.String, 0)
	case "hash":
		if len(entry.Hash) == 0 {
			return false, fmt.Errorf("%s 的 hash 内容为空，已跳过", entry.Key)
		}
		values := make([]interface{}, 0, len(entry.Hash)*2)
		for field, value := range entry.Hash {
			values = append(values, field, value)
		}
		pipe.HSet(ctx, entry.Key, values...)
	case "list":
		if len(entry.List) == 0 {
			return false, fmt.Errorf("%s 的 list 内容为空，已跳过", entry.Key)
		}
		values := make([]interface{}, len(entry.List))
		for i, value := range entry.List {
			values[i] = value
		}
		pipe.RPush(ctx, entry.Key, values...)
	case "set":
		if len(entry.Set) == 0 {
			return false, fmt.Errorf("%s 的 set 内容为空，已跳过", entry.Key)
		}
		values := make([]interface{}, len(entry.Set))
		for i, value := range entry.Set {
			values[i] = value
		}
		pipe.SAdd(ctx, entry.Key, values...)
	case "zset":
		if len(entry.ZSet) == 0 {
			return false, fmt.Errorf("%s 的 zset 内容为空，已跳过", entry.Key)
		}
		members := make([]redis.Z, len(entry.ZSet))
		for i, item := range entry.ZSet {
			members[i] = redis.Z{Score: item.Score, Member: item.Member}
		}
		pipe.ZAdd(ctx, entry.Key, members...)
	case "stream":
		if len(entry.Stream) == 0 {
			return false, fmt.Errorf("%s 的 stream 内容为空，已跳过", entry.Key)
		}
		for _, item := range entry.Stream {
			pipe.XAdd(ctx, &redis.XAddArgs{Stream: entry.Key, ID: item.ID, Values: item.Fields})
		}
	default:
		return false, fmt.Errorf("不支持写入的类型: %s", entry.Type)
	}

	if entry.TTL > 0 {
		pipe.Expire(ctx, entry.Key, time.Duration(entry.TTL)*time.Second)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return false, err
	}
	return true, nil
}

// GetDatabaseCount 读取实例配置的 DB 数量；集群或读取失败（云厂商常禁用 CONFIG）时回退 16
func GetDatabaseCount(ctx context.Context, client redis.UniversalClient) int {
	const fallback = 16
	single, ok := client.(*redis.Client)
	if !ok {
		return 1
	}
	values, err := single.ConfigGet(ctx, "databases").Result()
	if err != nil {
		return fallback
	}
	raw, ok := values["databases"]
	if !ok {
		return fallback
	}
	count, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || count <= 0 || count > 4096 {
		return fallback
	}
	return count
}

// CopyKeysToDB 在同一实例内把 key 复制到另一个 DB（依赖 Redis 6.2+ 的 COPY ... DB）
func CopyKeysToDB(ctx context.Context, client redis.UniversalClient, keys []string, dstDB int, overwrite bool) (int, int, int, error) {
	if _, ok := client.(*redis.ClusterClient); ok {
		return 0, 0, 0, fmt.Errorf("集群模式不支持跨 DB 复制")
	}
	if dstDB < 0 {
		return 0, 0, 0, fmt.Errorf("目标 DB 无效")
	}

	copied, skipped, failed := 0, 0, 0
	var firstErr error
	for start := 0; start < len(keys); start += batchKeyChunk {
		end := start + batchKeyChunk
		if end > len(keys) {
			end = len(keys)
		}
		chunk := keys[start:end]

		pipe := client.Pipeline()
		cmds := make([]*redis.IntCmd, len(chunk))
		for i, key := range chunk {
			cmds[i] = pipe.Copy(ctx, key, key, dstDB, overwrite)
		}
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			if firstErr == nil {
				firstErr = err
			}
		}
		for _, cmd := range cmds {
			value, err := cmd.Result()
			if err != nil {
				failed++
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if value == 1 {
				copied++
			} else {
				skipped++
			}
		}
	}
	if copied == 0 && failed > 0 && firstErr != nil {
		return copied, skipped, failed, firstErr
	}
	return copied, skipped, failed, nil
}

// FormatExportJSON 输出结构化 JSON
func FormatExportJSON(entries []config.ExportedKey) ([]byte, error) {
	return json.MarshalIndent(entries, "", "  ")
}

// FormatExportCommands 输出可直接 redis-cli 执行的命令
func FormatExportCommands(entries []config.ExportedKey) string {
	var builder strings.Builder
	builder.WriteString("# LiteRedis export\n")
	for _, entry := range entries {
		builder.WriteString("\n# " + entry.Key + " (" + entry.Type + ")\n")
		switch entry.Type {
		case "string":
			builder.WriteString("SET " + quoteRedisArg(entry.Key) + " " + quoteRedisArg(entry.String) + "\n")
		case "hash":
			fields := make([]string, 0, len(entry.Hash))
			for field := range entry.Hash {
				fields = append(fields, field)
			}
			sort.Strings(fields)
			for _, field := range fields {
				builder.WriteString("HSET " + quoteRedisArg(entry.Key) + " " + quoteRedisArg(field) + " " + quoteRedisArg(entry.Hash[field]) + "\n")
			}
		case "list":
			for _, value := range entry.List {
				builder.WriteString("RPUSH " + quoteRedisArg(entry.Key) + " " + quoteRedisArg(value) + "\n")
			}
		case "set":
			for _, value := range entry.Set {
				builder.WriteString("SADD " + quoteRedisArg(entry.Key) + " " + quoteRedisArg(value) + "\n")
			}
		case "zset":
			for _, item := range entry.ZSet {
				builder.WriteString("ZADD " + quoteRedisArg(entry.Key) + " " + fmt.Sprintf("%g", item.Score) + " " + quoteRedisArg(item.Member) + "\n")
			}
		case "stream":
			for _, item := range entry.Stream {
				parts := make([]string, 0, len(item.Fields)*2)
				fields := make([]string, 0, len(item.Fields))
				for field := range item.Fields {
					fields = append(fields, field)
				}
				sort.Strings(fields)
				for _, field := range fields {
					parts = append(parts, quoteRedisArg(field), quoteRedisArg(item.Fields[field]))
				}
				builder.WriteString("XADD " + quoteRedisArg(entry.Key) + " " + item.ID + " " + strings.Join(parts, " ") + "\n")
			}
		}
		if entry.TTL > 0 {
			builder.WriteString("EXPIRE " + quoteRedisArg(entry.Key) + " " + fmt.Sprintf("%d", entry.TTL) + "\n")
		}
	}
	return builder.String()
}

// quoteRedisArg 按 redis-cli 习惯给参数加引号并转义
func quoteRedisArg(value string) string {
	escaped := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	).Replace(value)
	return "\"" + escaped + "\""
}
