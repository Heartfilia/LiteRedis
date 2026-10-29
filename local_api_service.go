package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"LiteRedis/backend/config"
	"LiteRedis/backend/localapi"
	redisbackend "LiteRedis/backend/redis"
)

// localAPIQuery 把 /api/v1/* 的只读查询转接到现有 Redis 能力，不新增任何写入口
type localAPIQuery struct{ app *App }

func (q localAPIQuery) Connections() []localapi.ConnectionInfo {
	conns, err := q.app.ListConnections()
	if err != nil {
		return []localapi.ConnectionInfo{}
	}
	out := make([]localapi.ConnectionInfo, 0, len(conns))
	for _, item := range conns {
		connected := q.app.IsConnected(item.ID)
		db := item.DB
		if connected {
			if current, err := q.app.GetConnectionDB(item.ID); err == nil {
				db = current
			}
		}
		out = append(out, localapi.ConnectionInfo{
			ID:        item.ID,
			Name:      item.Name,
			Host:      item.Host,
			Port:      item.Port,
			DB:        db,
			IsCluster: item.IsCluster,
			ReadOnly:  item.ReadOnly,
			Connected: connected,
		})
	}
	return out
}

func (q localAPIQuery) CurrentDB(connID string) (int, bool) {
	if !q.app.IsConnected(connID) {
		return 0, false
	}
	db, err := q.app.GetConnectionDB(connID)
	if err != nil {
		return 0, false
	}
	return db, true
}

func (q localAPIQuery) Overview(connID string) (any, error) {
	return q.app.GetConnectionOverview(connID)
}

func (q localAPIQuery) Scan(connID, pattern string, count int64, cursor uint64) (any, error) {
	return q.app.ScanKeys(connID, pattern, count, cursor)
}

// Value 与界面里的读取同源，只是允许 API 指定每页条数
func (q localAPIQuery) Value(connID, key, keyType, zsetSort, streamStart string, cursor uint64, offset int, count int64) (any, error) {
	client, err := q.app.manager.GetClient(connID)
	if err != nil {
		return nil, err
	}
	settings, err := config.GetSettings()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		settings.HashLoadCount = count
		settings.ListLoadCount = count
		settings.SetLoadCount = count
		settings.ZSetLoadCount = count
		settings.StreamLoadCount = count
	}
	ctx, cancel := context.WithTimeout(q.app.ctx, 20*time.Second)
	defer cancel()
	_, db, err := q.app.manager.GetConnectionState(connID)
	if err != nil {
		return nil, err
	}
	scope := connID + "|" + strconv.Itoa(db)
	return redisbackend.GetValue(ctx, client, key, settings, cursor, offset, zsetSort, streamStart, scope, keyType)
}

// Command 只放行只读命令，写命令返回明确的拒绝错误
func (q localAPIQuery) Command(connID, command string) (any, error) {
	if !redisbackend.IsReadOnlyCommandAllowed(command) {
		return nil, localapi.CommandRejectedError{
			Message: fmt.Sprintf("只允许只读命令，已拒绝: %s", command),
		}
	}
	return q.app.ExecuteRedisCommand(connID, command)
}

// configureLocalAPI 把当前设置同步给本地 API 服务
func (a *App) configureLocalAPI(settings config.AppSettings) {
	a.apiServer.Configure(localAPIQuery{app: a}, localapi.Options{
		RedisEnabled: settings.LocalAPIRedisEnabled,
		AuthDisabled: settings.LocalAPIAuthDisabled,
	})
}
