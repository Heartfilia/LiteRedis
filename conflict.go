package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	redisbackend "LiteRedis/backend/redis"
)

// GetEditConflict returns a bounded, read-only comparison snapshot in the exact
// database where editing began. It does not follow workspace DB selection, check
// write permission, update connection state, or generate an edit token.
func (a *App) GetEditConflict(connID string, db int, key, field string, isHash bool) (redisbackend.ConflictSnapshot, error) {
	if strings.TrimSpace(connID) == "" || db < 0 || key == "" || (!isHash && field != "") {
		return redisbackend.ConflictSnapshot{}, fmt.Errorf("invalid conflict snapshot parameters")
	}
	if a.manager == nil {
		return redisbackend.ConflictSnapshot{}, fmt.Errorf("connection manager unavailable")
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return redisbackend.ConflictSnapshot{}, err
	}
	client, release, err := a.manager.NewOperationClientForDB(connID, db)
	if err != nil {
		return redisbackend.ConflictSnapshot{}, err
	}
	defer release()
	return redisbackend.ReadEditConflict(ctx, client, key, field, isHash)
}
