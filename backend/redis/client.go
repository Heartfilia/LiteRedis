package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"LiteRedis/backend/config"
	"LiteRedis/backend/ssh"

	"github.com/redis/go-redis/v9"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// activeConn 活跃连接
type activeConn struct {
	client        redis.UniversalClient
	sshClient     *gossh.Client
	sshForward    *ssh.LocalForward
	clusterDialer *ssh.ClusterForwardDialer
	forwardedAddr string
	cfg           config.ConnectionConfig
	currentDB     int
}

// ClientManager 连接池管理器
type ClientManager struct {
	mu                sync.RWMutex
	clients           map[string]*activeConn
	operationVersions map[string]uint64
}

const redisConnectTimeout = 10 * time.Second

// NewClientManager 创建连接池管理器
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:           make(map[string]*activeConn),
		operationVersions: make(map[string]uint64),
	}
}

func (m *ClientManager) nextOperationVersionLocked(id string) uint64 {
	next := m.operationVersions[id] + 1
	if next == 0 {
		next = 1
	}
	m.operationVersions[id] = next
	return next
}

// Connect 建立连接
func (m *ClientManager) Connect(cfg config.ConnectionConfig) error {
	start := time.Now()
	m.mu.Lock()
	operationVersion := m.nextOperationVersionLocked(cfg.ID)
	m.mu.Unlock()
	config.AppendDebugLog("[connect] begin id=%s name=%s redis=%s:%d cluster=%v ssh=%v", cfg.ID, cfg.Name, cfg.Host, cfg.Port, cfg.IsCluster, cfg.SSHEnabled)

	var sshClient *gossh.Client
	var sshForward *ssh.LocalForward
	var clusterDialer *ssh.ClusterForwardDialer
	var dialer func(ctx context.Context, network, addr string) (net.Conn, error)
	redisAddr := joinHostPort(cfg.Host, cfg.Port)

	if cfg.SSHEnabled && cfg.SSH != nil {
		config.AppendDebugLog("[connect] ssh enabled host=%s port=%d user=%s keyPath=%q", cfg.SSH.Host, cfg.SSH.Port, cfg.SSH.User, cfg.SSH.PrivateKeyPath)
		sc, err := ssh.NewSSHTunnelWithConfig(
			cfg.SSH.Host,
			cfg.SSH.Port,
			cfg.SSH.User,
			cfg.SSH.Password,
			cfg.SSH.PrivateKeyPath,
			cfg.SSH.Passphrase,
			remainingConnectTimeout(start),
		)
		if err != nil {
			config.AppendDebugLog("[connect] ssh setup failed: %v", err)
			return normalizeConnectError(fmt.Errorf("SSH tunnel error: %w", err))
		}
		sshClient = sc
		if cfg.IsCluster {
			clusterDialer = ssh.NewClusterForwardDialer(sc, redisConnectTimeout)
			dialer = clusterDialer.DialContext
		} else {
			forward, err := ssh.StartLocalForward(sc, redisAddr)
			if err != nil {
				sc.Close()
				config.AppendDebugLog("[connect] ssh local forward failed: %v", err)
				return normalizeConnectError(fmt.Errorf("SSH local forward error: %w", err))
			}
			sshForward = forward
			redisAddr = forward.Addr()
			config.AppendDebugLog("[connect] ssh local forward ready addr=%s", redisAddr)
		}
		config.AppendDebugLog("[connect] ssh ready")
	}
	if dialer == nil && cfg.ProxyEnabled && strings.TrimSpace(cfg.ProxyURL) != "" {
		pd, err := makeProxyDialer(cfg.ProxyURL, redisConnectTimeout)
		if err != nil {
			safeErr := redactProxyError(err, cfg.ProxyURL)
			config.AppendDebugLog("[connect] proxy setup failed: %v", safeErr)
			return normalizeConnectError(fmt.Errorf("Proxy error: %w", safeErr))
		}
		dialer = pd
		config.AppendDebugLog("[connect] proxy ready url=%s", redactProxyURL(cfg.ProxyURL))
	}

	var client redis.UniversalClient

	if cfg.IsCluster {
		addrs := normalizeAddrs(cfg.ClusterAddrs)
		if len(addrs) == 0 {
			addrs = []string{redisAddr}
		}
		opts := buildClusterOptions(addrs, cfg.Password, dialer)
		client = redis.NewClusterClient(opts)
	} else {
		opts := buildRedisOptions(redisAddr, cfg.Password, cfg.DB, dialer)
		client = redis.NewClient(opts)
	}

	// 测试连通性
	ctx, cancel := context.WithTimeout(context.Background(), remainingConnectTimeout(start))
	defer cancel()
	config.AppendDebugLog("[connect] redis ping begin")
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		if sshForward != nil {
			sshForward.Close()
		}
		if clusterDialer != nil {
			clusterDialer.Close()
		}
		if sshClient != nil {
			sshClient.Close()
		}
		config.AppendDebugLog("[connect] redis ping failed: %v", err)
		return normalizeConnectError(fmt.Errorf("Redis ping failed: %w", err))
	}
	newConn := &activeConn{
		client:        client,
		sshClient:     sshClient,
		sshForward:    sshForward,
		clusterDialer: clusterDialer,
		forwardedAddr: redisAddr,
		cfg:           cfg,
		currentDB:     cfg.DB,
	}
	m.mu.Lock()
	if m.operationVersions[cfg.ID] != operationVersion {
		m.mu.Unlock()
		closeActiveConnection(newConn)
		return fmt.Errorf("connection operation superseded")
	}
	old := m.clients[cfg.ID]
	m.clients[cfg.ID] = newConn
	m.mu.Unlock()
	closeActiveConnection(old)
	config.AppendDebugLog("[connect] success elapsed=%s", time.Since(start))
	return nil
}

// Disconnect 断开连接
func (m *ClientManager) Disconnect(id string) {
	m.mu.Lock()
	m.nextOperationVersionLocked(id)
	conn := m.clients[id]
	delete(m.clients, id)
	m.mu.Unlock()
	closeActiveConnection(conn)
}

// GetClient 获取指定连接的 Redis 客户端
func (m *ClientManager) GetClient(id string) (redis.UniversalClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.clients[id]
	if !ok {
		return nil, fmt.Errorf("connection %s not found or not connected", id)
	}
	return conn.client, nil
}

func (m *ClientManager) GetConnectionState(id string) (config.ConnectionConfig, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.clients[id]
	if !ok {
		return config.ConnectionConfig{}, 0, fmt.Errorf("connection %s not found or not connected", id)
	}
	return conn.cfg, conn.currentDB, nil
}

func (m *ClientManager) SetCurrentDB(id string, db int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.clients[id]
	if !ok {
		return fmt.Errorf("connection %s not found or not connected", id)
	}
	conn.currentDB = db
	conn.cfg.DB = db
	return nil
}

// ApplyConnectionConfig updates non-transport metadata in place. Transport
// changes invalidate the active client so it cannot keep using stale settings.
func (m *ClientManager) ApplyConnectionConfig(cfg config.ConnectionConfig) bool {
	m.mu.Lock()
	m.nextOperationVersionLocked(cfg.ID)
	conn, ok := m.clients[cfg.ID]
	if !ok {
		m.mu.Unlock()
		return false
	}
	if sameConnectionTransport(conn.cfg, cfg) {
		conn.cfg = cfg
		m.mu.Unlock()
		return false
	}
	delete(m.clients, cfg.ID)
	m.mu.Unlock()
	closeActiveConnection(conn)
	return true
}

func sameConnectionTransport(left, right config.ConnectionConfig) bool {
	return left.Host == right.Host &&
		left.Port == right.Port &&
		left.Password == right.Password &&
		left.DB == right.DB &&
		left.IsCluster == right.IsCluster &&
		reflect.DeepEqual(left.ClusterAddrs, right.ClusterAddrs) &&
		left.ProxyEnabled == right.ProxyEnabled &&
		left.ProxyURL == right.ProxyURL &&
		left.SSHEnabled == right.SSHEnabled &&
		reflect.DeepEqual(left.SSH, right.SSH)
}

func closeActiveConnection(conn *activeConn) {
	if conn == nil {
		return
	}
	if conn.client != nil {
		_ = conn.client.Close()
	}
	if conn.sshForward != nil {
		_ = conn.sshForward.Close()
	}
	if conn.clusterDialer != nil {
		_ = conn.clusterDialer.Close()
	}
	if conn.sshClient != nil {
		_ = conn.sshClient.Close()
	}
}

func cloneActiveConnection(conn *activeConn) *activeConn {
	if conn == nil {
		return nil
	}
	clone := *conn
	clone.cfg.ClusterAddrs = append([]string(nil), conn.cfg.ClusterAddrs...)
	if conn.cfg.SSH != nil {
		sshConfig := *conn.cfg.SSH
		clone.cfg.SSH = &sshConfig
	}
	return &clone
}

// IsConnected 检查连接是否存在
func (m *ClientManager) IsConnected(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[id]
	return ok
}

// Ping 检测现有连接是否仍可用
func (m *ClientManager) Ping(id string) error {
	m.mu.RLock()
	conn, ok := m.clients[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("connection %s not found or not connected", id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.client.Ping(ctx).Err(); err != nil {
		return normalizeConnectError(fmt.Errorf("Redis ping failed: %w", err))
	}
	return nil
}

// SelectDB 切换数据库（仅普通模式支持）
func (m *ClientManager) SelectDB(id string, db int) error {
	m.mu.Lock()
	conn, ok := m.clients[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("connection %s not found", id)
	}
	if conn.cfg.IsCluster {
		m.mu.Unlock()
		return fmt.Errorf("cluster mode does not support SELECT DB")
	}
	operationVersion := m.nextOperationVersionLocked(id)
	snapshot := cloneActiveConnection(conn)
	m.mu.Unlock()

	// 从原始配置重建，只改 DB，避免旧连接池污染。
	opts, err := buildActiveRedisOptions(snapshot, db)
	if err != nil {
		return err
	}

	newClient := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), redisConnectTimeout)
	defer cancel()
	if err := newClient.Ping(ctx).Err(); err != nil {
		newClient.Close()
		return fmt.Errorf("ping failed after SELECT: %w", err)
	}

	m.mu.Lock()
	if m.operationVersions[id] != operationVersion || m.clients[id] != conn {
		m.mu.Unlock()
		newClient.Close()
		return fmt.Errorf("select DB operation superseded")
	}
	oldClient := conn.client
	conn.client = newClient
	conn.currentDB = db
	conn.cfg.DB = db
	m.mu.Unlock()
	_ = oldClient.Close()
	return nil
}

func buildActiveRedisOptions(conn *activeConn, db int) (*redis.Options, error) {
	var dialer func(ctx context.Context, network, addr string) (net.Conn, error)
	redisAddr := joinHostPort(conn.cfg.Host, conn.cfg.Port)
	if conn.sshForward != nil {
		redisAddr = conn.forwardedAddr
	} else if conn.sshClient != nil {
		dialer = ssh.MakeContextDialer(conn.sshClient, redisConnectTimeout)
	} else if conn.cfg.ProxyEnabled && strings.TrimSpace(conn.cfg.ProxyURL) != "" {
		proxyDialer, err := makeProxyDialer(conn.cfg.ProxyURL, redisConnectTimeout)
		if err != nil {
			return nil, normalizeConnectError(fmt.Errorf("Proxy error: %w", redactProxyError(err, conn.cfg.ProxyURL)))
		}
		dialer = proxyDialer
	}
	return buildRedisOptions(redisAddr, conn.cfg.Password, db, dialer), nil
}

// TestConnection 测试连通性（不持久化，不保存连接池）
func TestConnection(cfg config.ConnectionConfig) error {
	start := time.Now()
	config.AppendDebugLog("[test] begin name=%s redis=%s:%d cluster=%v ssh=%v", cfg.Name, cfg.Host, cfg.Port, cfg.IsCluster, cfg.SSHEnabled)
	var sshClient *gossh.Client
	var sshForward *ssh.LocalForward
	var clusterDialer *ssh.ClusterForwardDialer
	var dialer func(ctx context.Context, network, addr string) (net.Conn, error)
	redisAddr := joinHostPort(cfg.Host, cfg.Port)

	if cfg.SSHEnabled && cfg.SSH != nil {
		config.AppendDebugLog("[test] ssh enabled host=%s port=%d user=%s keyPath=%q", cfg.SSH.Host, cfg.SSH.Port, cfg.SSH.User, cfg.SSH.PrivateKeyPath)
		sc, err := ssh.NewSSHTunnelWithConfig(
			cfg.SSH.Host,
			cfg.SSH.Port,
			cfg.SSH.User,
			cfg.SSH.Password,
			cfg.SSH.PrivateKeyPath,
			cfg.SSH.Passphrase,
			remainingConnectTimeout(start),
		)
		if err != nil {
			config.AppendDebugLog("[test] ssh setup failed: %v", err)
			return normalizeConnectError(fmt.Errorf("SSH tunnel error: %w", err))
		}
		sshClient = sc
		if cfg.IsCluster {
			clusterDialer = ssh.NewClusterForwardDialer(sc, redisConnectTimeout)
			dialer = clusterDialer.DialContext
		} else {
			forward, err := ssh.StartLocalForward(sc, redisAddr)
			if err != nil {
				sc.Close()
				config.AppendDebugLog("[test] ssh local forward failed: %v", err)
				return normalizeConnectError(fmt.Errorf("SSH local forward error: %w", err))
			}
			sshForward = forward
			redisAddr = forward.Addr()
			config.AppendDebugLog("[test] ssh local forward ready addr=%s", redisAddr)
		}
		defer sc.Close()
		if sshForward != nil {
			defer sshForward.Close()
		}
		if clusterDialer != nil {
			defer clusterDialer.Close()
		}
		config.AppendDebugLog("[test] ssh ready")
	}
	if dialer == nil && cfg.ProxyEnabled && strings.TrimSpace(cfg.ProxyURL) != "" {
		pd, err := makeProxyDialer(cfg.ProxyURL, redisConnectTimeout)
		if err != nil {
			safeErr := redactProxyError(err, cfg.ProxyURL)
			config.AppendDebugLog("[test] proxy setup failed: %v", safeErr)
			return normalizeConnectError(fmt.Errorf("Proxy error: %w", safeErr))
		}
		dialer = pd
		config.AppendDebugLog("[test] proxy ready url=%s", redactProxyURL(cfg.ProxyURL))
	}

	var client redis.UniversalClient

	if cfg.IsCluster {
		addrs := normalizeAddrs(cfg.ClusterAddrs)
		if len(addrs) == 0 {
			addrs = []string{redisAddr}
		}
		opts := buildClusterOptions(addrs, cfg.Password, dialer)
		client = redis.NewClusterClient(opts)
	} else {
		opts := buildRedisOptions(redisAddr, cfg.Password, cfg.DB, dialer)
		client = redis.NewClient(opts)
	}
	defer client.Close()
	_ = sshClient

	ctx, cancel := context.WithTimeout(context.Background(), remainingConnectTimeout(start))
	defer cancel()
	config.AppendDebugLog("[test] redis ping begin")
	err := client.Ping(ctx).Err()
	if err != nil {
		config.AppendDebugLog("[test] redis ping failed: %v", err)
		return normalizeConnectError(fmt.Errorf("Redis ping failed: %w", err))
	}
	config.AppendDebugLog("[test] success elapsed=%s", time.Since(start))
	return nil
}

// DisconnectAll 关闭所有连接（应用退出时调用）
func (m *ClientManager) DisconnectAll() {
	m.mu.Lock()
	connections := make([]*activeConn, 0, len(m.clients))
	for id, conn := range m.clients {
		m.nextOperationVersionLocked(id)
		connections = append(connections, conn)
		delete(m.clients, id)
	}
	m.mu.Unlock()
	for _, conn := range connections {
		closeActiveConnection(conn)
	}
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" || host == "localhost" {
		return "127.0.0.1"
	}
	return host
}

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(normalizeHost(host), strconv.Itoa(port))
}

func normalizeAddrs(addrs []string) []string {
	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		host, port, err := net.SplitHostPort(addr)
		if err == nil {
			result = append(result, net.JoinHostPort(normalizeHost(host), port))
			continue
		}
		result = append(result, addr)
	}
	return result
}

func makeProxyDialer(rawURL string, timeout time.Duration) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}
	baseDialer := &net.Dialer{Timeout: timeout}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := baseDialer.DialContext(ctx, network, parsed.Host)
			if err != nil {
				return nil, err
			}
			targetConn := conn
			connectHost := addr
			authHeader := ""
			if parsed.User != nil {
				username := parsed.User.Username()
				password, _ := parsed.User.Password()
				authHeader = "Proxy-Authorization: Basic " + basicProxyAuth(username, password) + "\r\n"
			}
			req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n%s\r\n", connectHost, connectHost, authHeader)
			if _, err := targetConn.Write([]byte(req)); err != nil {
				targetConn.Close()
				return nil, err
			}
			buf := make([]byte, 4096)
			n, err := targetConn.Read(buf)
			if err != nil {
				targetConn.Close()
				return nil, err
			}
			if !strings.Contains(string(buf[:n]), " 200 ") {
				targetConn.Close()
				return nil, fmt.Errorf("http proxy connect failed: %s", strings.TrimSpace(string(buf[:n])))
			}
			return targetConn, nil
		}, nil
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if parsed.User != nil {
			password, _ := parsed.User.Password()
			auth = &proxy.Auth{
				User:     parsed.User.Username(),
				Password: password,
			}
		}
		d, err := proxy.SOCKS5("tcp", parsed.Host, auth, baseDialer)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			type result struct {
				conn net.Conn
				err  error
			}
			done := make(chan result, 1)
			go func() {
				conn, err := d.Dial(network, addr)
				done <- result{conn: conn, err: err}
			}()
			select {
			case r := <-done:
				return r.conn, r.err
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", parsed.Scheme)
	}
}

func redactProxyURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "***"
	}
	redacted := strings.ToLower(parsed.Scheme) + "://"
	if parsed.User != nil {
		redacted += "***@"
	}
	return redacted + parsed.Host
}

func redactProxyError(err error, rawURL string) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL != "" {
		message = strings.ReplaceAll(message, trimmedURL, redactProxyURL(trimmedURL))
	}
	return errors.New(message)
}

func basicProxyAuth(username string, password string) string {
	plain := username + ":" + password
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out strings.Builder
	for i := 0; i < len(plain); i += 3 {
		var b0, b1, b2 byte
		b0 = plain[i]
		if i+1 < len(plain) {
			b1 = plain[i+1]
		}
		if i+2 < len(plain) {
			b2 = plain[i+2]
		}
		out.WriteByte(enc[b0>>2])
		out.WriteByte(enc[((b0&0x03)<<4)|(b1>>4)])
		if i+1 < len(plain) {
			out.WriteByte(enc[((b1&0x0f)<<2)|(b2>>6)])
		} else {
			out.WriteByte('=')
		}
		if i+2 < len(plain) {
			out.WriteByte(enc[b2&0x3f])
		} else {
			out.WriteByte('=')
		}
	}
	return out.String()
}

func buildRedisOptions(addr, password string, db int, dialer func(ctx context.Context, network, addr string) (net.Conn, error)) *redis.Options {
	opts := &redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  8 * time.Second,
		WriteTimeout: 8 * time.Second,
		PoolTimeout:  10 * time.Second,
		MaxRetries:   1,
	}
	if dialer != nil {
		opts.Dialer = dialer
	}
	return opts
}

func buildClusterOptions(addrs []string, password string, dialer func(ctx context.Context, network, addr string) (net.Conn, error)) *redis.ClusterOptions {
	opts := &redis.ClusterOptions{
		Addrs:        addrs,
		Password:     password,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  8 * time.Second,
		WriteTimeout: 8 * time.Second,
		PoolTimeout:  10 * time.Second,
		MaxRetries:   1,
	}
	if dialer != nil {
		opts.Dialer = dialer
		// SSH + Cluster 在 Windows 上更容易触发大量 socket 占用。
		// 这里主动压低每个节点的连接池和并发拨号规模，优先保证稳定性。
		opts.PoolSize = 2
		opts.MaxConcurrentDials = 1
		opts.MinIdleConns = 0
		opts.MaxIdleConns = 1
		opts.MaxActiveConns = 4
		opts.DialerRetries = 1
		opts.DialerRetryTimeout = 150 * time.Millisecond
		opts.ConnMaxIdleTime = 20 * time.Second
		opts.ConnMaxLifetime = 2 * time.Minute
	}
	return opts
}

func remainingConnectTimeout(start time.Time) time.Duration {
	remaining := redisConnectTimeout - time.Since(start)
	if remaining <= 0 {
		return time.Second
	}
	return remaining
}

func normalizeConnectError(err error) error {
	if err == nil {
		return nil
	}
	if isTimeoutError(err) {
		return fmt.Errorf("connection timed out after 10 seconds")
	}
	return err
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "timeout")
}
