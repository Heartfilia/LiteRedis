package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"LiteRedis/backend/config"
	redisbackend "LiteRedis/backend/redis"
)

type OperationFailure struct {
	Key     string `json:"key"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type OperationTask struct {
	ID           string             `json:"id"`
	Kind         string             `json:"kind"`
	ConnID       string             `json:"conn_id"`
	DB           int                `json:"db"`
	TargetConnID string             `json:"target_conn_id,omitempty"`
	TargetDB     int                `json:"target_db"`
	Total        int                `json:"total"`
	Processed    int                `json:"processed"`
	Succeeded    int                `json:"succeeded"`
	Skipped      int                `json:"skipped"`
	Failed       int                `json:"failed"`
	State        string             `json:"state"`
	Message      string             `json:"message,omitempty"`
	Failures     []OperationFailure `json:"failures"`
}

type operationRun struct {
	mu     sync.Mutex
	task   OperationTask
	ctx    context.Context
	cancel context.CancelFunc
}

func (a *App) beginOperation(kind, connID string, db int, target string, targetDB, total int) (*operationRun, error) {
	a.tasksMu.Lock()
	defer a.tasksMu.Unlock()
	for _, task := range a.tasks {
		if task.snapshot().State == "running" {
			return nil, errors.New("已有数据任务正在执行或停止，请等待其结束")
		}
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	run := &operationRun{ctx: ctx, cancel: cancel, task: OperationTask{
		ID: fmt.Sprintf("%d", time.Now().UnixNano()), Kind: kind, ConnID: connID, DB: db, TargetConnID: target, TargetDB: targetDB,
		Total: total, State: "running", Failures: []OperationFailure{},
	}}
	if len(a.tasks) >= 10 {
		a.tasks = a.tasks[len(a.tasks)-9:]
	}
	a.tasks = append(a.tasks, run)
	return run, nil
}

func (r *operationRun) snapshot() OperationTask {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot := r.task
	snapshot.Failures = append([]OperationFailure{}, r.task.Failures...)
	return snapshot
}

func (r *operationRun) step(key string, written bool, err error, writeAttempted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.task.Processed++
	if err != nil {
		r.task.Failed++
		code := redisbackend.RedisErrorCode(err)
		if !writeAttempted && code == "outcome_unknown" {
			code = "read_failed"
		}
		if len(r.task.Failures) < 100 {
			r.task.Failures = append(r.task.Failures, OperationFailure{Key: key, Code: code, Message: err.Error()})
		}
		if writeAttempted && (code == "outcome_unknown" || code == "partial_possible") {
			r.task.Message = "outcome_unknown"
			r.cancel()
		}
	} else if written {
		r.task.Succeeded++
	} else {
		r.task.Skipped++
	}
}

func (r *operationRun) finish(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.cancel()
	switch {
	case r.task.Message == "outcome_unknown":
		r.task.State = "outcome_unknown"
	case err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)):
		r.task.State = "cancelled"
	case err != nil || r.task.Failed > 0:
		r.task.State = "failed"
	default:
		r.task.State = "completed"
	}
	if err != nil && r.task.State != "outcome_unknown" {
		r.task.Message = err.Error()
	}
}

func (a *App) ListOperationTasks() []OperationTask {
	a.tasksMu.Lock()
	defer a.tasksMu.Unlock()
	result := make([]OperationTask, 0, len(a.tasks))
	for i := len(a.tasks) - 1; i >= 0; i-- {
		result = append(result, a.tasks[i].snapshot())
	}
	return result
}

func (a *App) CancelOperationTask(id string) bool {
	a.tasksMu.Lock()
	defer a.tasksMu.Unlock()
	for _, run := range a.tasks {
		if run.snapshot().ID == id {
			run.cancel()
			return true
		}
	}
	return false
}

// batch runs sequentially: cancellation stops future items, never rolls back writes.
func (r *operationRun) batch(keys []string, apply func(string) (bool, error, bool)) config.BatchKeyResult {
	var lastErr error
	for _, key := range keys {
		if err := r.ctx.Err(); err != nil {
			lastErr = err
			break
		}
		written, err, writeAttempted := apply(key)
		r.step(key, written, err, writeAttempted)
		if err != nil {
			lastErr = err
		}
	}
	if err := r.ctx.Err(); err != nil {
		lastErr = err
	}
	r.finish(lastErr)
	task := r.snapshot()
	message := fmt.Sprintf("成功 %d，跳过 %d，失败 %d（处理 %d/%d）", task.Succeeded, task.Skipped, task.Failed, task.Processed, task.Total)
	if task.State != "completed" {
		message += "；任务已停止或存在失败，已执行操作不会回滚。请查看任务清单"
	}
	code := ""
	if task.State == "outcome_unknown" {
		code = "outcome_unknown"
	}
	return config.BatchKeyResult{Success: task.State == "completed", Code: code, Affected: task.Succeeded, Failed: task.Failed, Message: message}
}
