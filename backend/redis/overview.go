package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

type ConsoleStateUpdater interface {
	SelectDB(id string, db int) error
}

func GetConnectionOverview(ctx context.Context, client redis.UniversalClient, cfg config.ConnectionConfig, currentDB int) (config.RedisConnectionOverview, error) {
	var infoMap map[string]string
	if clusterClient, ok := client.(*redis.ClusterClient); ok {
		var err error
		infoMap, err = aggregateClusterInfo(ctx, clusterClient)
		if err != nil {
			return config.RedisConnectionOverview{}, err
		}
	} else {
		info, err := client.Info(ctx).Result()
		if err != nil {
			return config.RedisConnectionOverview{}, err
		}
		infoMap = parseInfoText(info)
	}

	totalKeys, err := DBSize(ctx, client)
	if err != nil {
		return config.RedisConnectionOverview{}, err
	}
	usedMemoryBytes := parseInt64(infoMap["used_memory"])
	uptimeDays := parseInt64(infoMap["uptime_in_days"])
	if uptimeDays <= 0 {
		uptimeSeconds := parseInt64(infoMap["uptime_in_seconds"])
		uptimeDays = uptimeSeconds / 86400
	}

	overview := config.RedisConnectionOverview{
		ConnID:           cfg.ID,
		ConnName:         cfg.Name,
		Host:             cfg.Host,
		Port:             cfg.Port,
		CurrentDB:        currentDB,
		IsCluster:        cfg.IsCluster,
		RedisVersion:     infoMap["redis_version"],
		Role:             infoMap["role"],
		ConnectedClients: parseInt64(infoMap["connected_clients"]),
		InstantOpsPerSec: parseInt64(infoMap["instantaneous_ops_per_sec"]),
		TotalKeys:        totalKeys,
		UsedMemory:       firstNonEmpty(infoMap["used_memory_human"], formatBytes(usedMemoryBytes)),
		UsedMemoryBytes:  usedMemoryBytes,
		UptimeDays:       uptimeDays,
		UptimeHuman:      formatUptime(infoMap["uptime_in_seconds"]),
		MaxMemoryBytes:   parseInt64(infoMap["maxmemory"]),
		KeyspaceHits:     parseInt64(infoMap["keyspace_hits"]),
		KeyspaceMisses:   parseInt64(infoMap["keyspace_misses"]),
		EvictedKeys:      parseInt64(infoMap["evicted_keys"]),
		BlockedClients:   parseInt64(infoMap["blocked_clients"]),
		HitRate:          hitRate(parseInt64(infoMap["keyspace_hits"]), parseInt64(infoMap["keyspace_misses"])),
	}

	return overview, nil
}

// aggregateClusterInfo 汇总集群全部主节点的 INFO，保证概览与 TotalKeys 同口径：
// used_memory / instantaneous_ops_per_sec / connected_clients 求和，
// redis_version 取任一节点，uptime_in_seconds 取最大，role 固定 cluster。
// 不设置 used_memory_human，让上层回退到按求和字节数格式化。
func aggregateClusterInfo(ctx context.Context, client *redis.ClusterClient) (map[string]string, error) {
	var (
		nodeInfos []map[string]string
		firstErr  error
		nodes     int
	)
	err := client.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
		info, err := master.Info(ctx).Result()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		nodeInfos = append(nodeInfos, parseInfoText(info))
		nodes++
		return nil
	})
	if err != nil {
		return nil, err
	}
	if nodes == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("cluster has no reachable master nodes")
	}
	return mergeClusterNodeInfos(nodeInfos), nil
}

// mergeClusterNodeInfos 汇总多个节点的 INFO map：
// used_memory / instantaneous_ops_per_sec / connected_clients 求和，
// redis_version 取第一个非空值，uptime_in_seconds 取最大，role 固定 cluster。
func mergeClusterNodeInfos(nodeInfos []map[string]string) map[string]string {
	merged := map[string]string{
		"role": "cluster",
	}
	var (
		sumUsedMemory    int64
		sumOpsPerSec     int64
		sumConnected     int64
		sumHits          int64
		sumMisses        int64
		sumEvicted       int64
		sumBlocked       int64
		sumMaxMemory     int64
		maxUptimeSeconds int64
	)
	for _, nodeInfo := range nodeInfos {
		if merged["redis_version"] == "" {
			merged["redis_version"] = nodeInfo["redis_version"]
		}
		sumUsedMemory += parseInt64(nodeInfo["used_memory"])
		sumOpsPerSec += parseInt64(nodeInfo["instantaneous_ops_per_sec"])
		sumConnected += parseInt64(nodeInfo["connected_clients"])
		sumHits += parseInt64(nodeInfo["keyspace_hits"])
		sumMisses += parseInt64(nodeInfo["keyspace_misses"])
		sumEvicted += parseInt64(nodeInfo["evicted_keys"])
		sumBlocked += parseInt64(nodeInfo["blocked_clients"])
		sumMaxMemory += parseInt64(nodeInfo["maxmemory"])
		if uptime := parseInt64(nodeInfo["uptime_in_seconds"]); uptime > maxUptimeSeconds {
			maxUptimeSeconds = uptime
		}
	}
	merged["used_memory"] = strconv.FormatInt(sumUsedMemory, 10)
	merged["instantaneous_ops_per_sec"] = strconv.FormatInt(sumOpsPerSec, 10)
	merged["connected_clients"] = strconv.FormatInt(sumConnected, 10)
	merged["keyspace_hits"] = strconv.FormatInt(sumHits, 10)
	merged["keyspace_misses"] = strconv.FormatInt(sumMisses, 10)
	merged["evicted_keys"] = strconv.FormatInt(sumEvicted, 10)
	merged["blocked_clients"] = strconv.FormatInt(sumBlocked, 10)
	merged["maxmemory"] = strconv.FormatInt(sumMaxMemory, 10)
	merged["uptime_in_seconds"] = strconv.FormatInt(maxUptimeSeconds, 10)
	return merged
}

// hitRate 返回命中率（0~1），没有读写时返回 0
func hitRate(hits, misses int64) float64 {
	total := hits + misses
	if total <= 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func ExecuteRedisCommand(ctx context.Context, client redis.UniversalClient, command string) config.RedisConsoleResult {
	start := time.Now()
	args, err := splitCommand(command)
	if err != nil {
		return config.RedisConsoleResult{
			Success:   false,
			Command:   command,
			Error:     err.Error(),
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}
	if len(args) == 0 {
		return config.RedisConsoleResult{
			Success:   false,
			Command:   command,
			Error:     "empty command",
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}

	result, execErr := client.Do(ctx, args...).Result()
	elapsedMs := time.Since(start).Milliseconds()
	if execErr != nil {
		return config.RedisConsoleResult{
			Success:   false,
			Command:   command,
			Error:     execErr.Error(),
			ElapsedMs: elapsedMs,
		}
	}

	return config.RedisConsoleResult{
		Success:   true,
		Command:   command,
		Output:    formatRedisResult(result),
		ElapsedMs: elapsedMs,
	}
}

func ExecuteRedisCommandWithState(ctx context.Context, client redis.UniversalClient, command string, connID string, state ConsoleStateUpdater) config.RedisConsoleResult {
	args, err := splitCommand(command)
	if err != nil {
		return config.RedisConsoleResult{
			Success:   false,
			Command:   command,
			Error:     err.Error(),
			ElapsedMs: 0,
		}
	}
	if len(args) == 0 {
		return config.RedisConsoleResult{
			Success:   false,
			Command:   command,
			Error:     "empty command",
			ElapsedMs: 0,
		}
	}

	cmdName, ok := args[0].(string)
	if ok && strings.EqualFold(strings.TrimSpace(cmdName), "select") && state != nil {
		start := time.Now()
		if len(args) < 2 {
			return config.RedisConsoleResult{
				Success:   false,
				Command:   command,
				Error:     "ERR wrong number of arguments for 'select' command",
				ElapsedMs: time.Since(start).Milliseconds(),
			}
		}
		dbText, ok := args[1].(string)
		if !ok {
			return config.RedisConsoleResult{
				Success:   false,
				Command:   command,
				Error:     "ERR invalid DB index",
				ElapsedMs: time.Since(start).Milliseconds(),
			}
		}
		db, err := strconv.Atoi(strings.TrimSpace(dbText))
		if err != nil {
			return config.RedisConsoleResult{
				Success:   false,
				Command:   command,
				Error:     "ERR invalid DB index",
				ElapsedMs: time.Since(start).Milliseconds(),
			}
		}
		if err := state.SelectDB(connID, db); err != nil {
			return config.RedisConsoleResult{
				Success:   false,
				Command:   command,
				Error:     err.Error(),
				ElapsedMs: time.Since(start).Milliseconds(),
			}
		}
		return config.RedisConsoleResult{
			Success:   true,
			Command:   command,
			Output:    "OK",
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}

	return ExecuteRedisCommand(ctx, client, command)
}

func parseInfoText(info string) map[string]string {
	result := map[string]string{}
	lines := strings.Split(info, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		result[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return result
}

func parseInt64(value string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return n
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func formatBytes(size int64) string {
	if size <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	idx := 0
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}
	if idx == 0 {
		return fmt.Sprintf("%d %s", size, units[idx])
	}
	return fmt.Sprintf("%.2f %s", value, units[idx])
}

func formatUptime(rawSeconds string) string {
	seconds := parseInt64(rawSeconds)
	if seconds <= 0 {
		return "0m"
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func splitCommand(input string) ([]interface{}, error) {
	var args []interface{}
	var buf strings.Builder
	var quote rune
	escaped := false
	tokenStarted := false

	flush := func() {
		if !tokenStarted {
			return
		}
		args = append(args, buf.String())
		buf.Reset()
		tokenStarted = false
	}

	for _, ch := range input {
		if escaped {
			buf.WriteRune(ch)
			tokenStarted = true
			escaped = false
			continue
		}
		switch {
		case ch == '\\':
			escaped = true
			tokenStarted = true
		case quote != 0:
			if ch == quote {
				quote = 0
			} else {
				buf.WriteRune(ch)
			}
		case ch == '\'' || ch == '"':
			quote = ch
			tokenStarted = true
		case ch == ' ' || ch == '\t' || ch == '\n':
			flush()
		default:
			buf.WriteRune(ch)
			tokenStarted = true
		}
	}

	if escaped {
		buf.WriteRune('\\')
		tokenStarted = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}

	flush()
	return args, nil
}

func formatRedisResult(value interface{}) string {
	normalized := normalizeRedisValue(value)
	switch v := normalized.(type) {
	case nil:
		return "(nil)"
	case string:
		return v
	case []interface{}, map[string]interface{}:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	default:
		data, err := json.MarshalIndent(v, "", "  ")
		if err == nil && string(data) != "null" {
			return string(data)
		}
		return fmt.Sprintf("%v", v)
	}
}

func normalizeRedisValue(value interface{}) interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(v)
	case string, int64, int32, int, uint64, uint32, uint, float64, float32, bool:
		return v
	case []interface{}:
		items := make([]interface{}, 0, len(v))
		for _, item := range v {
			items = append(items, normalizeRedisValue(item))
		}
		return items
	case map[string]interface{}:
		items := make(map[string]interface{}, len(v))
		for key, item := range v {
			items[key] = normalizeRedisValue(item)
		}
		return items
	case map[interface{}]interface{}:
		items := make(map[string]interface{}, len(v))
		for key, item := range v {
			items[fmt.Sprintf("%v", key)] = normalizeRedisValue(item)
		}
		return items
	default:
		return fmt.Sprintf("%v", v)
	}
}
