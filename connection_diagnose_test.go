package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"time"

	redisbackend "LiteRedis/backend/redis"
	"strings"
	"testing"

	"LiteRedis/backend/config"
)

func TestAppDiagnoseConnectionWithIDCancelClosesRealSocket(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		_, _ = io.Copy(io.Discard, conn)
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	cfg := config.ConnectionConfig{Host: host, Port: portNumber}
	a := &App{}
	requestID := "cancel-real-socket"
	type result struct {
		report redisbackend.ConnectionDiagnosis
		err    error
	}
	finished := make(chan result, 1)
	go func() { r, err := a.DiagnoseConnectionWithID(requestID, cfg); finished <- result{r, err} }()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("diagnosis never connected")
	}
	if _, err := a.DiagnoseConnectionWithID(requestID, cfg); err == nil {
		t.Fatal("duplicate request ID accepted")
	}
	other := &App{}
	if other.CancelConnectionDiagnosis(requestID) {
		t.Fatal("another App cancelled this request")
	}
	if !a.CancelConnectionDiagnosis(requestID) {
		t.Fatal("active request not cancellable")
	}
	select {
	case got := <-finished:
		if got.err != nil {
			t.Fatal(got.err)
		}
		cancelled := false
		for _, stage := range got.report.Stages {
			if stage.Code == "cancelled" {
				cancelled = true
			}
		}
		if !cancelled {
			t.Fatalf("cancellation not reported: %+v", got.report)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop request")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("cancel left socket open")
	}
	if a.CancelConnectionDiagnosis(requestID) {
		t.Fatal("completed request retained")
	}
	// A completed ID can be reused; cleanup must not retain a stale run.
	if _, err := a.DiagnoseConnectionWithID(requestID, config.ConnectionConfig{}); err != nil {
		t.Fatal(err)
	}
}

func TestAppDiagnoseConnectionRequestIDValidation(t *testing.T) {
	a := &App{}
	for _, id := range []string{"", strings.Repeat("a", 101), "with space", "line\nfeed", "秘密"} {
		if _, err := a.DiagnoseConnectionWithID(id, config.ConnectionConfig{}); err == nil {
			t.Fatal("invalid request ID accepted")
		}
		if a.CancelConnectionDiagnosis(id) {
			t.Fatal("invalid request ID cancelled")
		}
	}
	if a.CancelConnectionDiagnosis("not-registered") {
		t.Fatal("unknown ID cancelled")
	}
}

func TestAppDiagnoseConnectionDoesNotTouchActiveConnection(t *testing.T) {
	a, mini := connectedWriteTestApp(t)
	mini.DB(4).Set("existing", "unchanged")
	before, db, err := a.manager.GetConnectionState("test")
	if err != nil {
		t.Fatal(err)
	}
	cfg := before
	cfg.ID = "test"
	cfg.DB = 7
	r, err := a.DiagnoseConnection(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, s := range r.Stages {
		if s.Stage == "redis_auth_ping" && s.Status == "success" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("diagnosis failed: %+v", r)
	}
	after, afterDB, err := a.manager.GetConnectionState("test")
	if err != nil || afterDB != db || after.DB != before.DB {
		t.Fatalf("active DB modified: %d %+v %v", afterDB, after, err)
	}
	if val, _ := mini.DB(4).Get("existing"); val != "unchanged" {
		t.Fatal("active data changed")
	}
}
func TestAppDiagnoseConnectionCancellationAndSanitizedReport(t *testing.T) {
	a := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.ctx = ctx
	cfg := config.ConnectionConfig{Host: "localhost", Port: 6379, Password: "never-print-secret"}
	r, err := a.DiagnoseConnection(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Stages) == 0 || r.Stages[0].Code != "cancelled" {
		t.Fatalf("ignored context: %+v", r)
	}
	data, _ := json.Marshal(r)
	if strings.Contains(string(data), cfg.Password) {
		t.Fatal("secret leaked")
	}
}
