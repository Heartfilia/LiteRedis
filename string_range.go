package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	redisbackend "LiteRedis/backend/redis"
)

// GetStringRangePreview reads one 64 KiB page from the explicitly selected DB
// using an isolated operation client, without changing workspace DB or obtaining
// write permission. No edit token is returned; this read is not a snapshot.
func (a *App) GetStringRangePreview(connID string, db int, key string, offset int64) (redisbackend.StringRangePreview, error) {
	if strings.TrimSpace(connID) == "" || db < 0 || key == "" || offset < 0 || offset > redisbackend.MaxStringRangeOffset {
		return redisbackend.StringRangePreview{}, fmt.Errorf("invalid string range preview parameters")
	}
	if a.manager == nil {
		return redisbackend.StringRangePreview{}, fmt.Errorf("connection manager unavailable")
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return redisbackend.StringRangePreview{}, err
	}
	client, release, err := a.manager.NewOperationClientForDB(connID, db)
	if err != nil {
		return redisbackend.StringRangePreview{}, err
	}
	defer release()
	return redisbackend.ReadStringRangePreview(ctx, client, key, offset)
}
