package redis

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"LiteRedis/backend/config"
	sshbackend "LiteRedis/backend/ssh"
	goredis "github.com/redis/go-redis/v9"
)

// ConnectionDiagnosis contains only static, non-sensitive diagnostic labels.
// A successful redis_auth_ping covers protocol negotiation, AUTH, SELECT and PING;
// it does not prove write permission or cluster topology health.
type ConnectionDiagnosis struct {
	Stages []ConnectionDiagnosticStage `json:"stages"`
}

type ConnectionDiagnosticStage struct {
	Stage      string `json:"stage"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Code       string `json:"code"`
	Hint       string `json:"hint"`
}

// DiagnoseConnection creates no manager entries and writes no connection config.
// Failures are carried in the sanitized report, never returned as raw errors.
// SSH uses only existing read-only trust records; clusters test one seed only.
func DiagnoseConnection(parent context.Context, cfg config.ConnectionConfig) (ConnectionDiagnosis, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	report := ConnectionDiagnosis{}
	for _, name := range []string{"config", "dns", "tcp", "proxy", "ssh", "tls", "redis_auth_ping"} {
		report.Stages = append(report.Stages, ConnectionDiagnosticStage{Stage: name, Status: "skipped", Code: "not_run", Hint: "Not executed because an earlier stage did not complete."})
	}
	set := func(stage, status, code, hint string, start time.Time) {
		for i := range report.Stages {
			if report.Stages[i].Stage == stage {
				report.Stages[i] = ConnectionDiagnosticStage{Stage: stage, Status: status, Code: code, Hint: hint, DurationMS: time.Since(start).Milliseconds()}
				return
			}
		}
	}
	fail := func(stage string, err error, start time.Time) (ConnectionDiagnosis, error) {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		code, hint := diagnosisFailure(err)
		set(stage, "failed", code, hint, start)
		return report, nil
	}
	start := time.Now()
	// Cluster forms may contain only seed addresses, without a standalone host.
	if cfg.IsCluster {
		if seeds := normalizeAddrs(cfg.ClusterAddrs); len(seeds) > 0 {
			host, port, err := net.SplitHostPort(seeds[0])
			if err != nil {
				set("config", "failed", "invalid_config", "Check the first cluster seed address.", start)
				return report, nil
			}
			parsedPort, err := strconv.Atoi(port)
			if err != nil {
				set("config", "failed", "invalid_config", "Check the first cluster seed port.", start)
				return report, nil
			}
			cfg.Host, cfg.Port, cfg.DB = host, parsedPort, 0
		}
	}
	if validateConnectionTransport(cfg) != nil || strings.TrimSpace(cfg.Host) == "" || cfg.Port < 1 || cfg.Port > 65535 || cfg.DB < 0 || strings.ContainsAny(cfg.Host, "\r\n\x00") {
		set("config", "failed", "invalid_config", "Check host, port, database and transport settings.", start)
		return report, nil
	}
	if err := ctx.Err(); err != nil {
		return fail("config", err, start)
	}
	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		set("config", "failed", "tls_config_invalid", "Check the configured CA certificate and TLS settings.", start)
		return report, nil
	}
	var dial func(context.Context, string, string) (net.Conn, error)
	if cfg.ProxyEnabled {
		dial, err = makeProxyDialer(cfg.ProxyURL, 15*time.Second)
		if err != nil {
			set("config", "failed", "proxy_config_invalid", "Check the proxy URL and supported proxy scheme.", start)
			return report, nil
		}
	}
	set("config", "success", "ok", "Connection settings validated; no configuration was saved.", start)
	addr := joinHostPort(cfg.Host, cfg.Port)
	if cfg.IsCluster {
		if seeds := normalizeAddrs(cfg.ClusterAddrs); len(seeds) > 0 {
			addr = seeds[0]
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			set("config", "failed", "invalid_config", "Check the first cluster seed address.", start)
			return report, nil
		}
		cfg.Host = host
		cfg.DB = 0
		if tlsCfg != nil {
			tlsCfg.ServerName = host
		}
	}
	stage := "tcp"
	if cfg.SSHEnabled {
		set("dns", "skipped", "remote_dns", "Target resolution is performed by the SSH server; local target DNS was not tested.", time.Now())
		set("proxy", "skipped", "not_enabled", "Proxy is not enabled.", time.Now())
		start = time.Now()
		sc, err := sshbackend.NewDiagnosticClient(ctx, *cfg.SSH)
		if err != nil {
			return fail("ssh", err, start)
		}
		defer sc.Close()
		set("ssh", "success", "combined_ssh", "SSH host DNS/TCP, existing fingerprint verification and authentication succeeded together.", start)
		dial = sc.DialContext
	} else if cfg.ProxyEnabled {
		set("ssh", "skipped", "not_enabled", "SSH is not enabled.", time.Now())
		stage = "proxy"
		set("dns", "skipped", "remote_dns", "Target DNS resolution belongs to the proxy path and was not tested locally.", time.Now())
		set("tcp", "skipped", "combined_proxy", "Proxy transport, target resolution and CONNECT/SOCKS negotiation are combined in the proxy stage.", time.Now())
	} else {
		set("ssh", "skipped", "not_enabled", "SSH is not enabled.", time.Now())
		set("proxy", "skipped", "not_enabled", "Proxy is not enabled.", time.Now())
		start = time.Now()
		if net.ParseIP(cfg.Host) == nil {
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, cfg.Host)
			if err != nil {
				return fail("dns", err, start)
			}
			if len(ips) == 0 {
				set("dns", "failed", "dns_failed", "No address was returned for the target host.", start)
				return report, nil
			}
			set("dns", "success", "ok", "Target host resolved locally.", start)
		} else {
			set("dns", "skipped", "ip_literal", "Target is an IP literal; no DNS lookup was needed.", start)
		}
		dial = (&net.Dialer{}).DialContext
	}
	start = time.Now()
	conn, err := dial(ctx, "tcp", addr)
	if err != nil {
		return fail(stage, err, start)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	set(stage, "success", "ok", "Transport connection established.", start)
	var transport net.Conn = conn
	if tlsCfg != nil {
		start = time.Now()
		tlsConn := tls.Client(conn, tlsCfg)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fail("tls", err, start)
		}
		transport = tlsConn
		hint := "TLS handshake and certificate validation succeeded."
		code := "ok"
		if cfg.TLSSkipVerify {
			hint = "TLS handshake succeeded with certificate verification disabled by configuration."
			code = "verification_disabled"
		}
		set("tls", "success", code, hint, start)
	} else {
		set("tls", "skipped", "not_enabled", "TLS is not enabled.", time.Now())
	}
	// Reuse the diagnosed transport exactly once. Never redial or replay, and
	// never run topology discovery or arbitrary user commands.
	var mu sync.Mutex
	used := false
	opts := buildRedisOptions(addr, cfg, cfg.DB, nil, nil)
	opts.Dialer = func(context.Context, string, string) (net.Conn, error) {
		mu.Lock()
		defer mu.Unlock()
		if used {
			return nil, errors.New("diagnostic transport already consumed")
		}
		used = true
		return transport, nil
	}
	opts.MaxRetries = -1
	opts.PoolSize = 1
	opts.MaxActiveConns = 1
	opts.ContextTimeoutEnabled = true
	opts.DialTimeout = 15 * time.Second
	opts.ReadTimeout = 15 * time.Second
	opts.WriteTimeout = 15 * time.Second
	client := goredis.NewClient(opts)
	defer client.Close()
	start = time.Now()
	if err := client.Ping(ctx).Err(); err != nil {
		return fail("redis_auth_ping", err, start)
	}
	if cfg.IsCluster {
		set("redis_auth_ping", "success", "seed_only", "Only the first cluster seed passed authentication and PING on DB 0; topology and other nodes were not checked.", start)
	} else {
		set("redis_auth_ping", "success", "ok", "Redis negotiation, configured authentication/database selection and PING succeeded.", start)
	}
	return report, nil
}

func diagnosisFailure(err error) (string, string) {
	if errors.Is(err, sshbackend.ErrDiagnosticUntrusted) || strings.Contains(err.Error(), "ssh_untrusted") {
		return "ssh_untrusted", "SSH host is not trusted; verify and approve its fingerprint using the connection flow first."
	}
	if errors.Is(err, sshbackend.ErrDiagnosticChanged) || strings.Contains(err.Error(), "ssh_changed") {
		return "ssh_changed", "SSH host fingerprint changed; diagnosis refused the connection without updating trust."
	}
	if errors.Is(err, sshbackend.ErrDiagnosticTrust) || strings.Contains(err.Error(), "ssh_trust_unavailable") {
		return "ssh_trust_unavailable", "Existing SSH trust records could not be read safely."
	}
	if strings.Contains(err.Error(), "ssh_key_invalid") {
		return "ssh_key_invalid", "Check the explicitly configured private key and passphrase."
	}
	if strings.Contains(err.Error(), "ssh_config_invalid") {
		return "ssh_config_invalid", "Check SSH host, port, username and explicit credentials."
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled", "Diagnosis was cancelled."
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", "Diagnosis exceeded its time limit."
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return "timeout", "The network operation timed out."
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns_failed", "Host resolution failed; check the hostname and DNS service."
	}
	message := strings.ToUpper(err.Error())
	if strings.Contains(message, "WRONGPASS") || strings.Contains(message, "NOAUTH") || strings.Contains(message, "AUTHENTICATION") {
		return "auth_failed", "Redis authentication failed; check credentials and ACL user status."
	}
	if strings.Contains(message, "NOPERM") {
		return "permission_denied", "The Redis user lacks permission for a required diagnostic command."
	}
	return "connection_failed", "The stage failed; check network, transport, certificate and server settings."
}
