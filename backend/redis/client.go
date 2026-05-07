package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
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
	client    redis.UniversalClient
	sshClient *gossh.Client
	cfg       config.ConnectionConfig
	currentDB int
}

// ClientManager 连接池管理器
type ClientManager struct {
	mu      sync.RWMutex
	clients map[string]*activeConn
}

const redisConnectTimeout = 10 * time.Second

// NewClientManager 创建连接池管理器
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*activeConn),
	}
}

// Connect 建立连接
func (m *ClientManager) Connect(cfg config.ConnectionConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	start := time.Now()
	config.AppendDebugLog("[connect] begin id=%s name=%s redis=%s:%d cluster=%v ssh=%v", cfg.ID, cfg.Name, cfg.Host, cfg.Port, cfg.IsCluster, cfg.SSHEnabled)

	// 已存在则先关闭
	if old, ok := m.clients[cfg.ID]; ok {
		old.client.Close()
		if old.sshClient != nil {
			old.sshClient.Close()
		}
		delete(m.clients, cfg.ID)
	}

	var sshClient *gossh.Client
	var dialer func(ctx context.Context, network, addr string) (net.Conn, error)

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
		dialer = ssh.MakeContextDialer(sc, redisConnectTimeout)
		config.AppendDebugLog("[connect] ssh ready")
	}
	if dialer == nil && cfg.ProxyEnabled && strings.TrimSpace(cfg.ProxyURL) != "" {
		pd, err := makeProxyDialer(cfg.ProxyURL, redisConnectTimeout)
		if err != nil {
			config.AppendDebugLog("[connect] proxy setup failed: %v", err)
			return normalizeConnectError(fmt.Errorf("Proxy error: %w", err))
		}
		dialer = pd
		config.AppendDebugLog("[connect] proxy ready url=%s", cfg.ProxyURL)
	}

	var client redis.UniversalClient

	if cfg.IsCluster {
		addrs := normalizeAddrs(cfg.ClusterAddrs)
		if len(addrs) == 0 {
			addrs = []string{joinHostPort(cfg.Host, cfg.Port)}
		}
		opts := buildClusterOptions(addrs, cfg.Password, dialer)
		client = redis.NewClusterClient(opts)
	} else {
		opts := buildRedisOptions(joinHostPort(cfg.Host, cfg.Port), cfg.Password, cfg.DB, dialer)
		client = redis.NewClient(opts)
	}

	// 测试连通性
	ctx, cancel := context.WithTimeout(context.Background(), remainingConnectTimeout(start))
	defer cancel()
	config.AppendDebugLog("[connect] redis ping begin")
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		if sshClient != nil {
			sshClient.Close()
		}
		config.AppendDebugLog("[connect] redis ping failed: %v", err)
		return normalizeConnectError(fmt.Errorf("Redis ping failed: %w", err))
	}
	config.AppendDebugLog("[connect] success elapsed=%s", time.Since(start))

	m.clients[cfg.ID] = &activeConn{
		client:    client,
		sshClient: sshClient,
		cfg:       cfg,
		currentDB: cfg.DB,
	}
	return nil
}

// Disconnect 断开连接
func (m *ClientManager) Disconnect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, ok := m.clients[id]; ok {
		conn.client.Close()
		if conn.sshClient != nil {
			conn.sshClient.Close()
		}
		delete(m.clients, id)
	}
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

// IsConnected 检查连接是否存在
func (m *ClientManager) IsConnected(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[id]
	return ok
}

// SelectDB 切换数据库（仅普通模式支持）
func (m *ClientManager) SelectDB(id string, db int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.clients[id]
	if !ok {
		return fmt.Errorf("connection %s not found", id)
	}
	if conn.cfg.IsCluster {
		return fmt.Errorf("cluster mode does not support SELECT DB")
	}

	// 从原始配置重建，只改 DB，避免旧连接池污染
	var dialer func(ctx context.Context, network, addr string) (net.Conn, error)
	if conn.sshClient != nil {
		dialer = ssh.MakeContextDialer(conn.sshClient, redisConnectTimeout)
	}

	opts := buildRedisOptions(joinHostPort(conn.cfg.Host, conn.cfg.Port), conn.cfg.Password, db, dialer)

	newClient := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), redisConnectTimeout)
	defer cancel()
	if err := newClient.Ping(ctx).Err(); err != nil {
		newClient.Close()
		return fmt.Errorf("ping failed after SELECT: %w", err)
	}

	conn.client.Close()
	conn.client = newClient
	conn.currentDB = db
	return nil
}

// TestConnection 测试连通性（不持久化，不保存连接池）
func TestConnection(cfg config.ConnectionConfig) error {
	start := time.Now()
	config.AppendDebugLog("[test] begin name=%s redis=%s:%d cluster=%v ssh=%v", cfg.Name, cfg.Host, cfg.Port, cfg.IsCluster, cfg.SSHEnabled)
	var sshClient *gossh.Client
	var dialer func(ctx context.Context, network, addr string) (net.Conn, error)

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
		dialer = ssh.MakeContextDialer(sc, redisConnectTimeout)
		defer sc.Close()
		config.AppendDebugLog("[test] ssh ready")
	}
	if dialer == nil && cfg.ProxyEnabled && strings.TrimSpace(cfg.ProxyURL) != "" {
		pd, err := makeProxyDialer(cfg.ProxyURL, redisConnectTimeout)
		if err != nil {
			config.AppendDebugLog("[test] proxy setup failed: %v", err)
			return normalizeConnectError(fmt.Errorf("Proxy error: %w", err))
		}
		dialer = pd
		config.AppendDebugLog("[test] proxy ready url=%s", cfg.ProxyURL)
	}

	var client redis.UniversalClient

	if cfg.IsCluster {
		addrs := normalizeAddrs(cfg.ClusterAddrs)
		if len(addrs) == 0 {
			addrs = []string{joinHostPort(cfg.Host, cfg.Port)}
		}
		opts := buildClusterOptions(addrs, cfg.Password, dialer)
		client = redis.NewClusterClient(opts)
	} else {
		opts := buildRedisOptions(joinHostPort(cfg.Host, cfg.Port), cfg.Password, cfg.DB, dialer)
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
	defer m.mu.Unlock()

	for id, conn := range m.clients {
		conn.client.Close()
		if conn.sshClient != nil {
			conn.sshClient.Close()
		}
		delete(m.clients, id)
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
	return fmt.Sprintf("%s:%d", normalizeHost(host), port)
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
