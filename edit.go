package main

import (
	"context"
	"time"

	"LiteRedis/backend/config"
	redisbackend "LiteRedis/backend/redis"
)

func (a *App) SetStringChecked(connID string, db int, key, value, token string) config.EditResult {
	return a.checkedEdit(connID, db, func(ctx context.Context, clientID string) config.EditResult {
		client, release, err := a.manager.NewOperationClientForDB(clientID, db)
		if err != nil {
			return config.EditResult{Code: "rejected", Message: err.Error()}
		}
		defer release()
		return redisbackend.SetStringChecked(ctx, client, key, value, token)
	})
}

func (a *App) HSetChecked(connID string, db int, key, field, value, expected string, existed bool) config.EditResult {
	return a.checkedEdit(connID, db, func(ctx context.Context, clientID string) config.EditResult {
		client, release, err := a.manager.NewOperationClientForDB(clientID, db)
		if err != nil {
			return config.EditResult{Code: "rejected", Message: err.Error()}
		}
		defer release()
		return redisbackend.HSetChecked(ctx, client, key, field, value, expected, existed)
	})
}

func (a *App) checkedEdit(connID string, db int, run func(context.Context, string) config.EditResult) config.EditResult {
	if db < 0 {
		return config.EditResult{Code: "invalid", Message: "Invalid database"}
	}
	if err := a.ensureWritable(connID); err != nil {
		return config.EditResult{Code: "rejected", Message: err.Error()}
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	return run(ctx, connID)
}
