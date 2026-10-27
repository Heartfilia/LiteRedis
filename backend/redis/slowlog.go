package redis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

const (
	// SlowLogDefaultCount 默认拉取的慢查询条数
	SlowLogDefaultCount int64 = 64
	// SlowLogMaxCount 单次最多拉取条数，避免一次拉太多
	SlowLogMaxCount int64 = 512
)

// GetSlowLog 读取慢查询；集群模式下汇总所有主节点并按耗时倒序
func GetSlowLog(ctx context.Context, client redis.UniversalClient, count int64) ([]config.SlowLogEntry, error) {
	if count <= 0 {
		count = SlowLogDefaultCount
	}
	if count > SlowLogMaxCount {
		count = SlowLogMaxCount
	}

	if clusterClient, ok := client.(*redis.ClusterClient); ok {
		entries := make([]config.SlowLogEntry, 0, count)
		var firstErr error
		_ = clusterClient.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
			list, err := master.SlowLogGet(ctx, count).Result()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return nil
			}
			addr := master.Options().Addr
			for _, item := range list {
				entries = append(entries, slowLogEntry(item, addr))
			}
			return nil
		})
		if len(entries) == 0 && firstErr != nil {
			return nil, firstErr
		}
		sortSlowLog(entries)
		return entries, nil
	}

	single, ok := client.(*redis.Client)
	if !ok {
		return nil, fmt.Errorf("unsupported redis client type")
	}
	list, err := single.SlowLogGet(ctx, count).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]config.SlowLogEntry, 0, len(list))
	for _, item := range list {
		entries = append(entries, slowLogEntry(item, ""))
	}
	sortSlowLog(entries)
	return entries, nil
}

func slowLogEntry(item redis.SlowLog, node string) config.SlowLogEntry {
	return config.SlowLogEntry{
		ID:         item.ID,
		Timestamp:  item.Time.Unix(),
		DurationUs: item.Duration.Microseconds(),
		Command:    strings.Join(item.Args, " "),
		ClientAddr: item.ClientAddr,
		ClientName: item.ClientName,
		Node:       node,
	}
}

// sortSlowLog 按耗时倒序，同耗时再按时间倒序
func sortSlowLog(entries []config.SlowLogEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].DurationUs != entries[j].DurationUs {
			return entries[i].DurationUs > entries[j].DurationUs
		}
		return entries[i].Timestamp > entries[j].Timestamp
	})
}
