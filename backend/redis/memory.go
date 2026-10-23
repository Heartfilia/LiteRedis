package redis

import (
	"container/heap"
	"context"
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
	// MemoryDefaultMaxKeys 单次体检最多扫描的 key 数量，避免误伤生产实例
	MemoryDefaultMaxKeys int64 = 200000
	// MemoryMaxKeysLimit 前端可请求的上限
	MemoryMaxKeysLimit int64 = 1000000
	// MemoryTopKeys 返回的大 key 明细条数
	MemoryTopKeys = 50
	// MemoryTopPrefixes 返回的前缀聚合条数
	MemoryTopPrefixes = 12
	// MemoryNoPrefixLabel 无命名前缀 key 的聚合标签
	MemoryNoPrefixLabel = "(no prefix)"

	memoryScanBatchSize = 200

	// SizeSourceMemory 服务端支持 MEMORY USAGE，大小最准
	SizeSourceMemory = "memory"
	// SizeSourceDebug 退化为 DEBUG OBJECT 的 serializedlength（近似值）
	SizeSourceDebug = "debug"
	// SizeSourceLength 再退化为元素数量，string 时为字节长度
	SizeSourceLength = "length"
)

// memorySizeMode 决定用哪种命令获取 key 大小，按服务端能力逐级降级
type memorySizeMode int

const (
	sizeModeMemory memorySizeMode = iota
	sizeModeDebug
	sizeModeLength
)

func (m memorySizeMode) source() string {
	switch m {
	case sizeModeDebug:
		return SizeSourceDebug
	case sizeModeLength:
		return SizeSourceLength
	default:
		return SizeSourceMemory
	}
}

// ScanMemory 用 SCAN 游标遍历 key，配合 pipeline 批量取 TYPE / MEMORY USAGE / PTTL，
// 输出大 key、前缀聚合和类型聚合。整个过程不持有全局锁，也不使用 KEYS。
func ScanMemory(ctx context.Context, client redis.UniversalClient, pattern string, maxKeys int64, onProgress func(scanned, total int64)) (config.MemoryReport, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = "*"
	}
	if maxKeys <= 0 {
		maxKeys = MemoryDefaultMaxKeys
	}
	if maxKeys > MemoryMaxKeysLimit {
		maxKeys = MemoryMaxKeysLimit
	}

	startedAt := time.Now()
	acc := newMemoryAccumulator()
	totalKeys, _ := DBSize(ctx, client)
	// 老版本 Redis（<4.0）没有 MEMORY USAGE，云厂商也可能禁用 DEBUG，
	// 先探测一次能力，再决定整轮扫描用哪种方式取大小
	mode := detectSizeMode(ctx, client, pattern)

	if clusterClient, ok := client.(*redis.ClusterClient); ok {
		var scanned int64
		err := clusterClient.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
			remaining := maxKeys - scanned
			if remaining <= 0 {
				return nil
			}
			nodeScanned, err := scanMemoryNode(ctx, master, pattern, remaining, mode, acc, func(delta int64) {
				if onProgress != nil {
					onProgress(scanned+delta, totalKeys)
				}
			})
			scanned += nodeScanned
			return err
		})
		if err != nil {
			return config.MemoryReport{}, err
		}
		acc.scanned = scanned
	} else {
		singleClient, ok := client.(*redis.Client)
		if !ok {
			return config.MemoryReport{}, fmt.Errorf("unsupported redis client type")
		}
		scanned, err := scanMemoryNode(ctx, singleClient, pattern, maxKeys, mode, acc, func(scanned int64) {
			if onProgress != nil {
				onProgress(scanned, totalKeys)
			}
		})
		if err != nil {
			return config.MemoryReport{}, err
		}
		acc.scanned = scanned
	}

	report := acc.build()
	report.Pattern = pattern
	report.TotalKeys = totalKeys
	report.SizeSource = mode.source()
	report.SizeUnavailable = mode == sizeModeLength
	report.ElapsedMs = time.Since(startedAt).Milliseconds()
	return report, nil
}

// scanMemoryNode 扫描单个节点，返回扫描到的 key 数量
func scanMemoryNode(ctx context.Context, client *redis.Client, pattern string, limit int64, mode memorySizeMode, acc *memoryAccumulator, onProgress func(int64)) (int64, error) {
	var (
		cursor  uint64
		scanned int64
	)
	for {
		if err := ctx.Err(); err != nil {
			return scanned, err
		}
		keys, nextCursor, err := client.Scan(ctx, cursor, pattern, memoryScanBatchSize).Result()
		if err != nil {
			return scanned, err
		}
		cursor = nextCursor

		remaining := limit - scanned
		if remaining <= 0 {
			acc.truncated = true
			return scanned, nil
		}
		// SCAN 的 COUNT 只是提示，单批可能超出上限，这里按上限截断
		if int64(len(keys)) > remaining {
			keys = keys[:remaining]
			acc.truncated = true
		}
		if len(keys) > 0 {
			if err := acc.addBatch(ctx, client, keys, mode); err != nil {
				return scanned, err
			}
			scanned += int64(len(keys))
			if onProgress != nil {
				onProgress(scanned)
			}
		}
		if acc.truncated || cursor == 0 {
			return scanned, nil
		}
	}
}

type memoryTypeAgg struct {
	keys  int64
	bytes int64
}

type memoryPrefixAgg struct {
	keys  int64
	bytes int64
}

type memoryAccumulator struct {
	scanned    int64
	bytes      int64
	noTTL      int64
	withSize   int64
	sizeErrors int64
	truncated  bool
	types      map[string]*memoryTypeAgg
	prefixes   map[string]*memoryPrefixAgg
	bigKeys    memoryKeyHeap
}

func newMemoryAccumulator() *memoryAccumulator {
	acc := &memoryAccumulator{
		types:    make(map[string]*memoryTypeAgg),
		prefixes: make(map[string]*memoryPrefixAgg),
	}
	heap.Init(&acc.bigKeys)
	return acc
}

// detectSizeMode 用第一个 key 探测服务端支持哪种大小查询方式
func detectSizeMode(ctx context.Context, client redis.UniversalClient, pattern string) memorySizeMode {
	keys, _, err := client.Scan(ctx, 0, pattern, 1).Result()
	if err != nil || len(keys) == 0 {
		// 空库或探测失败时保持默认，后续批次自然拿到空结果
		return sizeModeMemory
	}
	probe := keys[0]
	if err := client.MemoryUsage(ctx, probe).Err(); err == nil || errors.Is(err, redis.Nil) {
		return sizeModeMemory
	}
	if raw, err := client.Do(ctx, "DEBUG", "OBJECT", probe).Text(); err == nil {
		if _, ok := parseSerializedLength(raw); ok {
			return sizeModeDebug
		}
	}
	return sizeModeLength
}

// addBatch 按探测到的能力选择取大小的方式；单条命令失败不影响整体，
// 但整批全部失败通常意味着连接已断开，需要向上返回错误而不是产出空报告。
func (acc *memoryAccumulator) addBatch(ctx context.Context, client *redis.Client, keys []string, mode memorySizeMode) error {
	switch mode {
	case sizeModeDebug:
		return acc.addBatchDebug(ctx, client, keys)
	case sizeModeLength:
		return acc.addBatchLength(ctx, client, keys)
	default:
		return acc.addBatchMemory(ctx, client, keys)
	}
}

func (acc *memoryAccumulator) addBatchMemory(ctx context.Context, client *redis.Client, keys []string) error {
	pipe := client.Pipeline()
	typeCmds := make([]*redis.StatusCmd, len(keys))
	sizeCmds := make([]*redis.IntCmd, len(keys))
	ttlCmds := make([]*redis.DurationCmd, len(keys))
	for i, key := range keys {
		typeCmds[i] = pipe.Type(ctx, key)
		// 不指定 SAMPLES，走 Redis 默认采样（5），避免大集合全量遍历拖慢实例
		sizeCmds[i] = pipe.MemoryUsage(ctx, key)
		ttlCmds[i] = pipe.PTTL(ctx, key)
	}
	_, execErr := pipe.Exec(ctx)

	succeeded := 0
	samples := make([]memoryKeySample, 0, len(keys))
	for i, key := range keys {
		keyType, ok := readKeyType(typeCmds[i])
		if !ok {
			continue
		}
		succeeded++
		size, sizeErr := sizeCmds[i].Result()
		if sizeErr != nil || size < 0 {
			acc.sizeErrors++
			size = 0
		} else {
			acc.withSize++
		}
		samples = append(samples, memoryKeySample{key: key, keyType: keyType, size: size, ttl: readTTL(ttlCmds[i])})
	}
	if succeeded == 0 && len(keys) > 0 && execErr != nil && !errors.Is(execErr, redis.Nil) {
		return execErr
	}
	acc.consume(samples)
	return nil
}

// addBatchDebug DEBUG OBJECT 的 serializedlength 是近似内存，兼容 Redis 2.6+
func (acc *memoryAccumulator) addBatchDebug(ctx context.Context, client *redis.Client, keys []string) error {
	pipe := client.Pipeline()
	typeCmds := make([]*redis.StatusCmd, len(keys))
	debugCmds := make([]*redis.Cmd, len(keys))
	ttlCmds := make([]*redis.DurationCmd, len(keys))
	for i, key := range keys {
		typeCmds[i] = pipe.Type(ctx, key)
		debugCmds[i] = pipe.Do(ctx, "DEBUG", "OBJECT", key)
		ttlCmds[i] = pipe.PTTL(ctx, key)
	}
	_, execErr := pipe.Exec(ctx)

	succeeded := 0
	samples := make([]memoryKeySample, 0, len(keys))
	for i, key := range keys {
		keyType, ok := readKeyType(typeCmds[i])
		if !ok {
			continue
		}
		succeeded++
		size := int64(0)
		parsed := false
		if raw, err := debugCmds[i].Text(); err == nil {
			size, parsed = parseSerializedLength(raw)
		}
		if parsed {
			acc.withSize++
		} else {
			acc.sizeErrors++
		}
		samples = append(samples, memoryKeySample{key: key, keyType: keyType, size: size, ttl: readTTL(ttlCmds[i])})
	}
	if succeeded == 0 && len(keys) > 0 && execErr != nil && !errors.Is(execErr, redis.Nil) {
		return execErr
	}
	acc.consume(samples)
	return nil
}

// addBatchLength 最后的降级方案：只取元素数量，string 取字节长度
func (acc *memoryAccumulator) addBatchLength(ctx context.Context, client *redis.Client, keys []string) error {
	pipe := client.Pipeline()
	typeCmds := make([]*redis.StatusCmd, len(keys))
	ttlCmds := make([]*redis.DurationCmd, len(keys))
	for i, key := range keys {
		typeCmds[i] = pipe.Type(ctx, key)
		ttlCmds[i] = pipe.PTTL(ctx, key)
	}
	_, execErr := pipe.Exec(ctx)

	type keyMeta struct {
		key     string
		keyType string
		ttl     int64
	}
	metas := make([]keyMeta, 0, len(keys))
	for i, key := range keys {
		keyType, ok := readKeyType(typeCmds[i])
		if !ok {
			continue
		}
		metas = append(metas, keyMeta{key: key, keyType: keyType, ttl: readTTL(ttlCmds[i])})
	}
	if len(metas) == 0 && len(keys) > 0 && execErr != nil && !errors.Is(execErr, redis.Nil) {
		return execErr
	}

	lenPipe := client.Pipeline()
	lenCmds := make([]*redis.IntCmd, len(metas))
	for i, meta := range metas {
		switch meta.keyType {
		case "string":
			lenCmds[i] = lenPipe.StrLen(ctx, meta.key)
		case "hash":
			lenCmds[i] = lenPipe.HLen(ctx, meta.key)
		case "list":
			lenCmds[i] = lenPipe.LLen(ctx, meta.key)
		case "set":
			lenCmds[i] = lenPipe.SCard(ctx, meta.key)
		case "zset":
			lenCmds[i] = lenPipe.ZCard(ctx, meta.key)
		case "stream":
			lenCmds[i] = lenPipe.XLen(ctx, meta.key)
		}
	}
	_, _ = lenPipe.Exec(ctx)

	samples := make([]memoryKeySample, 0, len(metas))
	for i, meta := range metas {
		length := int64(0)
		if lenCmds[i] != nil {
			if value, err := lenCmds[i].Result(); err == nil && value > 0 {
				length = value
			}
		}
		samples = append(samples, memoryKeySample{key: meta.key, keyType: meta.keyType, length: length, ttl: meta.ttl})
	}
	acc.consume(samples)
	return nil
}

type memoryKeySample struct {
	key     string
	keyType string
	size    int64
	length  int64
	ttl     int64
}

func (acc *memoryAccumulator) consume(samples []memoryKeySample) {
	for _, sample := range samples {
		acc.bytes += sample.size
		if sample.ttl < 0 {
			acc.noTTL++
		}
		acc.recordType(sample.keyType, sample.size)
		acc.recordPrefix(memoryPrefixOf(sample.key), sample.size)
		acc.pushBigKey(config.MemoryKeyInfo{
			Key:  sample.key,
			Type: sample.keyType,
			Size: sample.size,
			Len:  sample.length,
			TTL:  sample.ttl,
		})
	}
}

func readKeyType(cmd *redis.StatusCmd) (string, bool) {
	keyType, err := cmd.Result()
	if err != nil || keyType == "" || keyType == "none" {
		return "", false
	}
	return keyType, true
}

func readTTL(cmd *redis.DurationCmd) int64 {
	ttl, err := cmd.Result()
	if err != nil {
		return -1
	}
	if ttl < 0 {
		return -1
	}
	seconds := int64(ttl / time.Second)
	if seconds < 0 {
		return 0
	}
	return seconds
}

// parseSerializedLength 从 DEBUG OBJECT 输出里取出 serializedlength
func parseSerializedLength(raw string) (int64, bool) {
	const prefix = "serializedlength:"
	idx := strings.Index(raw, prefix)
	if idx < 0 {
		return 0, false
	}
	rest := raw[idx+len(prefix):]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	value, err := strconv.ParseInt(rest[:end], 10, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func (acc *memoryAccumulator) recordType(keyType string, size int64) {
	agg, ok := acc.types[keyType]
	if !ok {
		agg = &memoryTypeAgg{}
		acc.types[keyType] = agg
	}
	agg.keys++
	agg.bytes += size
}

func (acc *memoryAccumulator) recordPrefix(prefix string, size int64) {
	agg, ok := acc.prefixes[prefix]
	if !ok {
		agg = &memoryPrefixAgg{}
		acc.prefixes[prefix] = agg
	}
	agg.keys++
	agg.bytes += size
}

func (acc *memoryAccumulator) pushBigKey(info config.MemoryKeyInfo) {
	if len(acc.bigKeys) < MemoryTopKeys {
		heap.Push(&acc.bigKeys, info)
		return
	}
	if info.Weight() <= acc.bigKeys[0].Weight() {
		return
	}
	heap.Pop(&acc.bigKeys)
	heap.Push(&acc.bigKeys, info)
}

func (acc *memoryAccumulator) build() config.MemoryReport {
	report := config.MemoryReport{
		ScannedKeys: acc.scanned,
		TotalBytes:  acc.bytes,
		NoTTLKeys:   acc.noTTL,
		Truncated:   acc.truncated,
		Types:       make([]config.MemoryTypeStat, 0, len(acc.types)),
		Prefixes:    make([]config.MemoryPrefixStat, 0, len(acc.prefixes)),
		BigKeys:     make([]config.MemoryKeyInfo, 0, len(acc.bigKeys)),
	}
	if acc.withSize > 0 {
		report.AvgKeyBytes = acc.bytes / acc.withSize
	}
	for keyType, agg := range acc.types {
		report.Types = append(report.Types, config.MemoryTypeStat{Type: keyType, Keys: agg.keys, Bytes: agg.bytes})
	}
	sort.Slice(report.Types, func(i, j int) bool {
		if report.Types[i].Bytes != report.Types[j].Bytes {
			return report.Types[i].Bytes > report.Types[j].Bytes
		}
		return report.Types[i].Type < report.Types[j].Type
	})

	for prefix, agg := range acc.prefixes {
		report.Prefixes = append(report.Prefixes, config.MemoryPrefixStat{Prefix: prefix, Keys: agg.keys, Bytes: agg.bytes})
	}
	sort.Slice(report.Prefixes, func(i, j int) bool {
		if report.Prefixes[i].Bytes != report.Prefixes[j].Bytes {
			return report.Prefixes[i].Bytes > report.Prefixes[j].Bytes
		}
		return report.Prefixes[i].Keys > report.Prefixes[j].Keys
	})
	if len(report.Prefixes) > MemoryTopPrefixes {
		report.Prefixes = report.Prefixes[:MemoryTopPrefixes]
	}

	for acc.bigKeys.Len() > 0 {
		report.BigKeys = append(report.BigKeys, heap.Pop(&acc.bigKeys).(config.MemoryKeyInfo))
	}
	sort.Slice(report.BigKeys, func(i, j int) bool { return report.BigKeys[i].Weight() > report.BigKeys[j].Weight() })

	return report
}

// memoryPrefixOf 取 key 的第一个 ":" 前缀；无前缀的 key 归入统一分组
func memoryPrefixOf(key string) string {
	idx := strings.Index(key, ":")
	if idx <= 0 {
		return MemoryNoPrefixLabel
	}
	prefix := key[:idx]
	if prefix == "" {
		return MemoryNoPrefixLabel
	}
	return prefix
}

// memoryKeyHeap 维护 Top N 大 key 的小顶堆
type memoryKeyHeap []config.MemoryKeyInfo

func (h memoryKeyHeap) Len() int            { return len(h) }
func (h memoryKeyHeap) Less(i, j int) bool  { return h[i].Weight() < h[j].Weight() }
func (h memoryKeyHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *memoryKeyHeap) Push(x interface{}) { *h = append(*h, x.(config.MemoryKeyInfo)) }
func (h *memoryKeyHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
