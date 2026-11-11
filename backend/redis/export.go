package redis

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	// 单 key 原始数据预算（不含 Go 容器开销）；项数限制也防止大量空值占用内存。
	ExportMaxKeyBytes = 32 << 20
	ExportMaxKeyItems = 100000
	// 与当前 JSON 导入的 64MiB 文件上限保持一致。
	ExportMaxFileBytes = 64 << 20
	exportPageSize     = 200
	exportStringChunk  = 256 << 10
)

// ReadKeyForExport 按类型把 key 的完整内容读出来
func ReadKeyForExport(ctx context.Context, client redis.UniversalClient, key string) (config.ExportedKey, error) {
	return readKeyForExport(ctx, client, key, ExportMaxKeyBytes, ExportMaxKeyItems)
}

// 分页不是一致性快照：导出期间应暂停源数据写入。
func readKeyForExport(ctx context.Context, client redis.UniversalClient, key string, maxBytes, maxItems int) (config.ExportedKey, error) {
	info, err := exportKeyInfo(ctx, client, key)
	if err != nil {
		return config.ExportedKey{}, err
	}
	entry := config.ExportedKey{Key: key, Type: info.Type, TTL: info.TTL}
	used, items := len(key), 0
	add := func(n, count int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if n > maxBytes-used || count > maxItems-items {
			return fmt.Errorf("key %q 超过导出上限（%d 字节 / %d 项），请拆分后重试", key, maxBytes, maxItems)
		}
		used += n
		items += count
		return nil
	}
	if err := add(0, 0); err != nil {
		return entry, err
	}
	if info.Count > int64(maxItems) {
		return entry, fmt.Errorf("key %q 超过 %d 项导出上限", key, maxItems)
	}

	switch info.Type {
	case "string":
		size, err := client.StrLen(ctx, key).Result()
		if err != nil {
			return entry, err
		}
		if size > int64(ExportMaxStringBytes) || size > int64(maxBytes-used) {
			return entry, fmt.Errorf("key %q 的字符串超过导出字节上限", key)
		}
		var value strings.Builder
		value.Grow(int(size))
		for start := int64(0); start < size; start += exportStringChunk {
			if err := ctx.Err(); err != nil {
				return entry, err
			}
			end := min(start+exportStringChunk, size) - 1
			part, err := client.GetRange(ctx, key, start, end).Result()
			if err != nil {
				return entry, err
			}
			if int64(len(part)) != end-start+1 {
				return entry, fmt.Errorf("key %q 在导出期间发生变化", key)
			}
			if err := add(len(part), 0); err != nil {
				return entry, err
			}
			value.WriteString(part)
		}
		entry.String = value.String()
	case "hash":
		entry.Hash = make(map[string]string)
		var cursor uint64
		for {
			if err := ctx.Err(); err != nil {
				return entry, err
			}
			values, next, err := client.HScan(ctx, key, cursor, "*", exportPageSize).Result()
			if err != nil {
				return entry, err
			}
			for i := 0; i < len(values); i += 2 {
				if _, exists := entry.Hash[values[i]]; exists {
					continue
				}
				if err := add(len(values[i])+len(values[i+1]), 1); err != nil {
					return entry, err
				}
				entry.Hash[values[i]] = values[i+1]
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	case "list":
		for start := int64(0); start < info.Count; start += exportPageSize {
			if err := ctx.Err(); err != nil {
				return entry, err
			}
			values, err := client.LRange(ctx, key, start, min(start+exportPageSize, info.Count)-1).Result()
			if err != nil {
				return entry, err
			}
			for _, value := range values {
				if err := add(len(value), 1); err != nil {
					return entry, err
				}
				entry.List = append(entry.List, value)
			}
		}
	case "set":
		seen := make(map[string]struct{})
		var cursor uint64
		for {
			if err := ctx.Err(); err != nil {
				return entry, err
			}
			values, next, err := client.SScan(ctx, key, cursor, "*", exportPageSize).Result()
			if err != nil {
				return entry, err
			}
			for _, value := range values {
				if _, exists := seen[value]; exists {
					continue
				}
				if err := add(len(value), 1); err != nil {
					return entry, err
				}
				seen[value] = struct{}{}
				entry.Set = append(entry.Set, value)
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
		sort.Strings(entry.Set)
	case "zset":
		for start := int64(0); start < info.Count; start += exportPageSize {
			if err := ctx.Err(); err != nil {
				return entry, err
			}
			values, err := client.ZRangeWithScores(ctx, key, start, min(start+exportPageSize, info.Count)-1).Result()
			if err != nil {
				return entry, err
			}
			for _, item := range values {
				member := fmt.Sprint(item.Member)
				if err := add(len(member)+8, 1); err != nil {
					return entry, err
				}
				entry.ZSet = append(entry.ZSet, config.ZSetMember{Member: member, Score: item.Score})
			}
		}
	case "stream":
		// 固定末尾 ID，持续 XADD 不会让导出永远追赶新消息。
		tail, err := client.XRevRangeN(ctx, key, "+", "-", 1).Result()
		if err != nil {
			return entry, err
		}
		if len(tail) == 0 {
			break
		}
		start, end := "-", tail[0].ID
		for {
			if err := ctx.Err(); err != nil {
				return entry, err
			}
			values, err := client.XRangeN(ctx, key, start, end, exportPageSize).Result()
			if err != nil {
				return entry, err
			}
			if len(values) == 0 {
				break
			}
			for _, item := range values {
				if err := add(len(item.ID), 1); err != nil {
					return entry, err
				}
				fields := make(map[string]string, len(item.Values))
				for field, raw := range item.Values {
					value := fmt.Sprint(raw)
					if err := add(len(field)+len(value), 1); err != nil {
						return entry, err
					}
					fields[field] = value
				}
				entry.Stream = append(entry.Stream, config.StreamEntry{ID: item.ID, Fields: fields})
			}
			last := values[len(values)-1].ID
			if last == end {
				break
			}
			start = "(" + last
		}
	default:
		return entry, fmt.Errorf("不支持导出的类型: %s", info.Type)
	}
	// 检测删除、类型变化和数量变化；等长原地修改仍可能无法察觉。
	current, err := exportKeyInfo(ctx, client, key)
	if err != nil {
		return entry, err
	}
	if current.Type != entry.Type || current.Count != info.Count {
		return entry, fmt.Errorf("key %q 在导出期间发生变化，请暂停写入后重试", key)
	}
	var count int
	switch entry.Type {
	case "string":
		size, err := client.StrLen(ctx, key).Result()
		if err != nil {
			return entry, err
		}
		if size != int64(len(entry.String)) {
			return entry, fmt.Errorf("key %q 在导出期间发生变化", key)
		}
	case "hash":
		count = len(entry.Hash)
	case "list":
		count = len(entry.List)
	case "set":
		count = len(entry.Set)
	case "zset":
		count = len(entry.ZSet)
	case "stream":
		count = len(entry.Stream)
	}
	if entry.Type != "string" && int64(count) != info.Count {
		return entry, fmt.Errorf("key %q 在导出期间发生变化，请暂停写入后重试", key)
	}
	return entry, ctx.Err()
}

// 只查询当前类型的长度并检查每条命令的错误，不能把 ACL 拒绝当作长度 0。
func exportKeyInfo(ctx context.Context, client redis.UniversalClient, key string) (config.RedisKey, error) {
	kind, err := client.Type(ctx, key).Result()
	if err != nil {
		return config.RedisKey{}, err
	}
	pipe := client.Pipeline()
	ttl := pipe.TTL(ctx, key)
	var count *redis.IntCmd
	switch kind {
	case "string": // STRLEN 在读取 string 内容前单独查询。
	case "hash":
		count = pipe.HLen(ctx, key)
	case "list":
		count = pipe.LLen(ctx, key)
	case "set":
		count = pipe.SCard(ctx, key)
	case "zset":
		count = pipe.ZCard(ctx, key)
	case "stream":
		count = pipe.XLen(ctx, key)
	default:
		return config.RedisKey{}, fmt.Errorf("key %q 不存在或不支持导出的类型: %s", key, kind)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return config.RedisKey{}, err
	}
	info := config.RedisKey{Name: key, Type: kind, TTL: int64(ttl.Val())}
	if ttl.Val() >= 0 {
		info.TTL = int64(ttl.Val() / time.Second)
	}
	if info.TTL == -2 {
		return info, fmt.Errorf("key %q 在导出期间已删除", key)
	}
	if count != nil {
		info.Count = count.Val()
	}
	return info, nil
}

// ExportKeysToWriter 逐 key 编码，不在内存里构建整个导出文件。
// maxBytes 限制编码后的文件大小（含 JSON 转义），0 使用默认 64MiB。
func ExportKeysToWriter(ctx context.Context, client redis.UniversalClient, keys []string, format string, out io.Writer, maxBytes int64) error {
	return ExportKeysToWriterProgress(ctx, client, keys, format, out, maxBytes, nil)
}

func ExportKeysToWriterProgress(ctx context.Context, client redis.UniversalClient, keys []string, format string, out io.Writer, maxBytes int64, onKey func(string)) error {
	if len(keys) == 0 || len(keys) > ExportMaxKeys {
		return fmt.Errorf("导出 key 数必须为 1 到 %d", ExportMaxKeys)
	}
	if maxBytes <= 0 {
		maxBytes = ExportMaxFileBytes
	}
	w := &exportBudgetWriter{ctx: ctx, out: out, remaining: maxBytes}
	commands := strings.EqualFold(format, "commands")
	if commands {
		if _, err := io.WriteString(w, "# LiteRedis export\n"); err != nil {
			return err
		}
	} else if _, err := io.WriteString(w, "[\n"); err != nil {
		return err
	}
	// 命令导出按行写入，缓冲避免每个元素都触发磁盘系统调用。
	buffer := bufio.NewWriterSize(w, 64<<10)
	encoder := json.NewEncoder(buffer)
	for i, key := range keys {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, err := ReadKeyForExport(ctx, client, key)
		if err != nil {
			return &ExportKeyError{Key: key, Err: err}
		}
		if commands {
			err = writeExportCommands(buffer, []config.ExportedKey{entry}, false)
		} else {
			if i > 0 {
				if _, err := io.WriteString(buffer, ",\n"); err != nil {
					return err
				}
			}
			err = encoder.Encode(entry)
		}
		if err != nil {
			return err
		}
		if err := buffer.Flush(); err != nil {
			return err
		}
		if onKey != nil {
			onKey(key)
		}
	}
	if !commands {
		if _, err := io.WriteString(buffer, "]\n"); err != nil {
			return err
		}
	}
	if err := buffer.Flush(); err != nil {
		return err
	}
	return ctx.Err()
}

type ExportKeyError struct {
	Key string
	Err error
}

func (e *ExportKeyError) Error() string { return fmt.Sprintf("导出 %q: %v", e.Key, e.Err) }
func (e *ExportKeyError) Unwrap() error { return e.Err }

type exportBudgetWriter struct {
	ctx       context.Context
	out       io.Writer
	remaining int64
}

func (w *exportBudgetWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("导出文件超过大小上限，请减少选择的 key 后重试")
	}
	n, err := w.out.Write(p)
	w.remaining -= int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

// WriteExportedKey 把导出的 key 写回（用于跨实例复制）。
// overwrite=false 时目标已存在则跳过，返回 false。
func WriteExportedKey(ctx context.Context, client redis.UniversalClient, entry config.ExportedKey, overwrite bool) (bool, error) {
	if entry.Key == "" || entry.TTL == -2 {
		return false, nil
	}
	// 所有内容先校验，不能先删目标再发现导入数据无效。
	if err := validateExportedKey(entry); err != nil {
		return false, &InvalidExportError{Err: err}
	}
	return writeStagedExport(ctx, client, entry, overwrite)
}

type InvalidExportError struct{ Err error }

func (e *InvalidExportError) Error() string { return e.Err.Error() }
func (e *InvalidExportError) Unwrap() error { return e.Err }

func validateExportedKey(entry config.ExportedKey) error {
	bytes, items := len(entry.Key), 0
	add := func(n, count int) { bytes += n; items += count }
	switch entry.Type {
	case "string":
		add(len(entry.String), 1)
	case "hash":
		for field, value := range entry.Hash {
			add(len(field)+len(value), 1)
		}
	case "list":
		for _, value := range entry.List {
			add(len(value), 1)
		}
	case "set":
		for _, value := range entry.Set {
			add(len(value), 1)
		}
	case "zset":
		for _, value := range entry.ZSet {
			add(len(value.Member)+8, 1)
		}
	case "stream":
		for _, value := range entry.Stream {
			if len(value.Fields) > 1000 {
				return fmt.Errorf("%s 的单条 Stream 消息超过 1000 个字段，无法安全构建", entry.Key)
			}
			add(len(value.ID), 1)
			for field, text := range value.Fields {
				add(len(field)+len(text), 1)
			}
		}
	}
	if bytes > ImportMaxKeyBytes || items > ImportMaxKeyItems {
		return fmt.Errorf("%s 超过单 key 安全导入上限（5MiB / 1 万项）；请拆分数据", entry.Key)
	}
	if entry.TTL < -2 || entry.TTL > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("%s 的 TTL 无效", entry.Key)
	}
	empty := false
	switch entry.Type {
	case "string":
		if len(entry.String) > ExportMaxStringBytes {
			return fmt.Errorf("%s 的字符串超过 32MB", entry.Key)
		}
	case "hash":
		empty = len(entry.Hash) == 0
	case "list":
		empty = len(entry.List) == 0
	case "set":
		empty = len(entry.Set) == 0
	case "zset":
		empty = len(entry.ZSet) == 0
		for _, item := range entry.ZSet {
			if math.IsNaN(item.Score) || math.IsInf(item.Score, 0) {
				return fmt.Errorf("%s 的 zset score 无效", entry.Key)
			}
		}
	case "stream":
		empty = len(entry.Stream) == 0
		var prevMS, prevSeq uint64
		for _, item := range entry.Stream {
			msText, seqText, ok := strings.Cut(item.ID, "-")
			ms, msErr := strconv.ParseUint(msText, 10, 64)
			seq, seqErr := strconv.ParseUint(seqText, 10, 64)
			if !ok || msErr != nil || seqErr != nil || len(item.Fields) == 0 || ms < prevMS || (ms == prevMS && seq <= prevSeq) {
				return fmt.Errorf("%s 的 stream ID 或字段无效", entry.Key)
			}
			prevMS, prevSeq = ms, seq
		}
	default:
		return fmt.Errorf("不支持写入的类型: %s", entry.Type)
	}
	if empty {
		return fmt.Errorf("%s 的 %s 内容为空，已跳过", entry.Key, entry.Type)
	}
	return nil
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
	_ = writeExportCommands(&builder, entries, true)
	return builder.String()
}

type exportTextWriter struct {
	out io.Writer
	err error
}

func (w *exportTextWriter) WriteString(s string) {
	if w.err == nil {
		_, w.err = io.WriteString(w.out, s)
	}
}

func writeExportCommands(out io.Writer, entries []config.ExportedKey, header bool) error {
	builder := &exportTextWriter{out: out}
	if header {
		builder.WriteString("# LiteRedis export\n")
	}
	for _, entry := range entries {
		if builder.err != nil {
			return builder.err
		}
		builder.WriteString("\n# " + quoteRedisArg(entry.Key) + " (" + quoteRedisArg(entry.Type) + ")\n")
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
				if builder.err != nil {
					return builder.err
				}
				builder.WriteString("HSET " + quoteRedisArg(entry.Key) + " " + quoteRedisArg(field) + " " + quoteRedisArg(entry.Hash[field]) + "\n")
			}
		case "list":
			for _, value := range entry.List {
				if builder.err != nil {
					return builder.err
				}
				builder.WriteString("RPUSH " + quoteRedisArg(entry.Key) + " " + quoteRedisArg(value) + "\n")
			}
		case "set":
			for _, value := range entry.Set {
				if builder.err != nil {
					return builder.err
				}
				builder.WriteString("SADD " + quoteRedisArg(entry.Key) + " " + quoteRedisArg(value) + "\n")
			}
		case "zset":
			for _, item := range entry.ZSet {
				if builder.err != nil {
					return builder.err
				}
				builder.WriteString("ZADD " + quoteRedisArg(entry.Key) + " " + fmt.Sprintf("%g", item.Score) + " " + quoteRedisArg(item.Member) + "\n")
			}
		case "stream":
			for _, item := range entry.Stream {
				if builder.err != nil {
					return builder.err
				}
				parts := make([]string, 0, len(item.Fields)*2)
				fields := make([]string, 0, len(item.Fields))
				for field := range item.Fields {
					fields = append(fields, field)
				}
				sort.Strings(fields)
				for _, field := range fields {
					parts = append(parts, quoteRedisArg(field), quoteRedisArg(item.Fields[field]))
				}
				builder.WriteString("XADD " + quoteRedisArg(entry.Key) + " " + quoteRedisArg(item.ID) + " " + strings.Join(parts, " ") + "\n")
			}
		}
		if entry.TTL > 0 {
			builder.WriteString("EXPIRE " + quoteRedisArg(entry.Key) + " " + fmt.Sprintf("%d", entry.TTL) + "\n")
		}
	}
	return builder.err
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
