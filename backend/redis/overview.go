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
	info, err := client.Info(ctx).Result()
	if err != nil {
		return config.RedisConnectionOverview{}, err
	}

	infoMap := parseInfoText(info)
	totalKeys, _ := DBSize(ctx, client)
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
	}

	return overview, nil
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

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		args = append(args, buf.String())
		buf.Reset()
	}

	for _, ch := range input {
		if escaped {
			buf.WriteRune(ch)
			escaped = false
			continue
		}
		switch {
		case ch == '\\':
			escaped = true
		case quote != 0:
			if ch == quote {
				quote = 0
			} else {
				buf.WriteRune(ch)
			}
		case ch == '\'' || ch == '"':
			quote = ch
		case ch == ' ' || ch == '\t' || ch == '\n':
			flush()
		default:
			buf.WriteRune(ch)
		}
	}

	if escaped {
		buf.WriteRune('\\')
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
