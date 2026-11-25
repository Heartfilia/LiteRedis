package redis

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"LiteRedis/backend/config"
	"github.com/alicebob/miniredis/v2"
)

func diagnosisConfig(addr string) config.ConnectionConfig {
	host, port, _ := net.SplitHostPort(addr)
	n, _ := strconv.Atoi(port)
	return config.ConnectionConfig{Host: host, Port: n}
}
func diagnosticStage(t *testing.T, r ConnectionDiagnosis, name string) ConnectionDiagnosticStage {
	t.Helper()
	for _, s := range r.Stages {
		if s.Stage == name {
			return s
		}
	}
	t.Fatalf("missing stage %s", name)
	return ConnectionDiagnosticStage{}
}
func TestDiagnoseConnectionSuccessAndAuthSanitized(t *testing.T) {
	mini := miniredis.RunT(t)
	cfg := diagnosisConfig(mini.Addr())
	r, err := DiagnoseConnection(context.Background(), cfg)
	if err != nil || diagnosticStage(t, r, "tcp").Status != "success" || diagnosticStage(t, r, "redis_auth_ping").Status != "success" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	if diagnosticStage(t, r, "dns").Status != "skipped" || diagnosticStage(t, r, "tls").Status != "skipped" {
		t.Fatal("unexecuted stages claimed success")
	}
	mini.RequireAuth("expected-password-secret")
	cfg.Password = "incorrect-password-secret"
	cfg.Username = "private-user-secret"
	r, err = DiagnoseConnection(context.Background(), cfg)
	if err != nil || diagnosticStage(t, r, "redis_auth_ping").Status != "failed" {
		t.Fatalf("auth report=%+v %v", r, err)
	}
	data, _ := json.Marshal(r)
	for _, secret := range []string{cfg.Password, cfg.Username, "expected-password-secret"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("credentials leaked")
		}
	}
}
func TestDiagnoseConnectionUnsupportedAndValidation(t *testing.T) {
	for _, cfg := range []config.ConnectionConfig{
		{Host: "localhost", Port: 0},
		{Host: "localhost", Port: 6379, ProxyEnabled: true, ProxyURL: "garbage://user:secret@localhost:1234"},
		{Host: "localhost", Port: 6379, SSHEnabled: true, SSH: &config.SSHConfig{}},
	} {
		r, err := DiagnoseConnection(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		failed := false
		for _, stage := range r.Stages {
			if stage.Status == "failed" {
				failed = true
			}
		}
		if !failed {
			t.Fatalf("missing failure: %+v", r)
		}
		if diagnosticStage(t, r, "tcp").Status != "skipped" {
			t.Fatal("performed network for unsupported/invalid config")
		}
	}
}
func TestDiagnoseConnectionClusterSeedOnly(t *testing.T) {
	mini := miniredis.RunT(t)
	cfg := diagnosisConfig(mini.Addr())
	cfg.IsCluster = true
	cfg.ClusterAddrs = []string{mini.Addr(), "unreachable.invalid:6379"}
	cfg.DB = 4
	r, err := DiagnoseConnection(context.Background(), cfg)
	if err != nil || diagnosticStage(t, r, "redis_auth_ping").Code != "seed_only" {
		t.Fatalf("seed diagnosis: %+v %v", r, err)
	}
}

func TestDiagnoseConnectionTLSCertificateFailureIsSanitized(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	cfg := diagnosisConfig(server.Listener.Addr().String())
	cfg.TLSEnabled = true
	r, err := DiagnoseConnection(context.Background(), cfg)
	if err != nil || diagnosticStage(t, r, "tls").Status != "failed" || diagnosticStage(t, r, "redis_auth_ping").Status != "skipped" {
		t.Fatalf("TLS failure misreported: %+v %v", r, err)
	}
}

func TestDiagnoseConnectionCancellationClosesTransport(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		_, _ = io.Copy(io.Discard, conn)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-accepted; cancel() }()
	start := time.Now()
	r, err := DiagnoseConnection(ctx, diagnosisConfig(listener.Addr().String()))
	if err != nil || diagnosticStage(t, r, "redis_auth_ping").Status != "failed" || time.Since(start) > 2*time.Second {
		t.Fatalf("cancellation not bounded: %+v %v", r, err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("transport left open")
	}
}
func TestDiagnoseConnectionProxyCombinedWithoutTargetDNS(t *testing.T) {
	mini := miniredis.RunT(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	target := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		target <- line
		for {
			line, err = reader.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
		}
		upstream, err := net.Dial("tcp", mini.Addr())
		if err != nil {
			return
		}
		defer upstream.Close()
		fmt.Fprint(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
		copyDone := make(chan struct{})
		go func() { io.Copy(upstream, reader); upstream.Close(); close(copyDone) }()
		io.Copy(conn, upstream)
		conn.Close()
		<-copyDone
	}()
	cfg := config.ConnectionConfig{Host: "remote-only.invalid", Port: 6379, ProxyEnabled: true, ProxyURL: "http://user:proxy-secret@" + listener.Addr().String()}
	r, err := DiagnoseConnection(context.Background(), cfg)
	if err != nil || diagnosticStage(t, r, "proxy").Status != "success" || diagnosticStage(t, r, "redis_auth_ping").Status != "success" {
		t.Fatalf("proxy report=%+v err=%v", r, err)
	}
	if diagnosticStage(t, r, "dns").Code != "remote_dns" || diagnosticStage(t, r, "tcp").Status != "skipped" {
		t.Fatal("misleading proxy staging")
	}
	if request := <-target; !strings.Contains(request, "remote-only.invalid:6379") {
		t.Fatal("target resolved locally")
	}
	data, _ := json.Marshal(r)
	if strings.Contains(string(data), "proxy-secret") {
		t.Fatal("proxy credential leak")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("proxy resource leak")
	}
}

func TestDiagnoseConnectionClusterSeedWithoutStandaloneHost(t *testing.T) {
	mini := miniredis.RunT(t)
	cfg := config.ConnectionConfig{IsCluster: true, ClusterAddrs: []string{mini.Addr()}}
	report, err := DiagnoseConnection(context.Background(), cfg)
	if err != nil || diagnosticStage(t, report, "redis_auth_ping").Code != "seed_only" {
		t.Fatalf("seed diagnosis failed: %+v %v", report, err)
	}
}
