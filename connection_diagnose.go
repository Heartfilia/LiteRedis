package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"LiteRedis/backend/config"
	redisbackend "LiteRedis/backend/redis"
)

type connectionDiagnosisKey struct {
	app *App
	id  string
}

type connectionDiagnosisRun struct{ cancel context.CancelFunc }

var connectionDiagnosisRegistry = struct {
	sync.Mutex
	runs map[connectionDiagnosisKey]*connectionDiagnosisRun
}{runs: make(map[connectionDiagnosisKey]*connectionDiagnosisRun)}

func validDiagnosisRequestID(id string) bool {
	if len(id) == 0 || len(id) > 100 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// DiagnoseConnectionWithID registers a bounded cancellable run before doing any
// network work. Duplicate request IDs are rejected until their old run exits.
// IDs are scoped to an App, so one App cannot cancel another App's diagnosis.
func (a *App) DiagnoseConnectionWithID(requestID string, cfg config.ConnectionConfig) (redisbackend.ConnectionDiagnosis, error) {
	if !validDiagnosisRequestID(requestID) {
		return redisbackend.ConnectionDiagnosis{}, errors.New("invalid diagnosis request ID")
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	run := &connectionDiagnosisRun{cancel: cancel}
	key := connectionDiagnosisKey{app: a, id: requestID}
	connectionDiagnosisRegistry.Lock()
	if _, exists := connectionDiagnosisRegistry.runs[key]; exists {
		connectionDiagnosisRegistry.Unlock()
		cancel()
		return redisbackend.ConnectionDiagnosis{}, errors.New("diagnosis request already running")
	}
	connectionDiagnosisRegistry.runs[key] = run
	connectionDiagnosisRegistry.Unlock()
	defer func() {
		cancel()
		connectionDiagnosisRegistry.Lock()
		if connectionDiagnosisRegistry.runs[key] == run {
			delete(connectionDiagnosisRegistry.runs, key)
		}
		connectionDiagnosisRegistry.Unlock()
	}()
	return redisbackend.DiagnoseConnection(ctx, cfg)
}

// CancelConnectionDiagnosis returns false if the request has not registered yet
// or already completed. Callers may retry cancellation after starting invocation;
// this API deliberately does not retain pre-cancel tombstones or unbounded IDs.
func (a *App) CancelConnectionDiagnosis(requestID string) bool {
	if !validDiagnosisRequestID(requestID) {
		return false
	}
	connectionDiagnosisRegistry.Lock()
	run := connectionDiagnosisRegistry.runs[connectionDiagnosisKey{app: a, id: requestID}]
	connectionDiagnosisRegistry.Unlock()
	if run == nil {
		return false
	}
	run.cancel()
	return true
}

// DiagnoseConnection is isolated from the active connection manager and storage.
// The backend owns a 15-second context and closes in-flight sockets on cancellation.
func (a *App) DiagnoseConnection(cfg config.ConnectionConfig) (redisbackend.ConnectionDiagnosis, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return redisbackend.DiagnoseConnection(ctx, cfg)
}
