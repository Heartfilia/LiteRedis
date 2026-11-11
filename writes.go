package main

import (
	"context"
	"fmt"
	"time"

	"LiteRedis/backend/config"
	redisbackend "LiteRedis/backend/redis"
	"github.com/redis/go-redis/v9"
)

// writeClient pins the DB before execution and never retries a possibly executed write.
func (a *App) writeClient(connID string) (context.Context, redis.UniversalClient, func(), error) {
	if err := a.ensureWritable(connID); err != nil {
		return nil, nil, nil, err
	}
	return a.operationClient(connID, 30*time.Second)
}

func (a *App) operationClient(connID string, timeout time.Duration) (context.Context, redis.UniversalClient, func(), error) {
	client, release, err := a.manager.NewOperationClientForDB(connID, -1)
	if err != nil {
		return nil, nil, nil, err
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return ctx, client, func() { cancel(); release() }, nil
}

func writeResult(err error) config.OperationResult {
	if err == nil {
		return config.OperationResult{Success: true}
	}
	code := redisbackend.RedisErrorCode(err)
	message := err.Error()
	if code == "outcome_unknown" || code == "partial_possible" {
		message = "写入结果未知：服务端可能已经执行。请保留输入并刷新核实，不要直接重复提交。\n" + err.Error()
	}
	return config.OperationResult{Code: code, Message: message}
}

func (a *App) runWrite(connID string, apply func(context.Context, redis.UniversalClient) error) config.OperationResult {
	ctx, client, done, err := a.writeClient(connID)
	if err != nil {
		return config.OperationResult{Code: "rejected", Message: err.Error()}
	}
	defer done()
	defer redisbackend.InvalidateConnectionListChunks(connID)
	return writeResult(apply(ctx, client))
}

func batchWriteResult(affected, total int, err error) config.BatchKeyResult {
	result := writeResult(err)
	message := result.Message
	if err != nil {
		message = fmt.Sprintf("已确认影响 %d 个；其余 %d 个未执行、失败或结果未知。%s", affected, max(0, total-affected), message)
	}
	return config.BatchKeyResult{Success: result.Success, Code: result.Code, Affected: affected, Failed: max(0, total-affected), Message: message}
}
