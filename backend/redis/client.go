package redis

import (
	"context"
	"fmt"
	"net"
	"sync"

	"LiteRedis/backend/config"
	"LiteRedis/backend/ssh"

	"github.com/redis/go-redis/v9"
	gossh "golang.org/x/crypto/ssh"
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

	// 已存在则先关闭
	if old, ok := m.clients[cfg.ID]; ok {
		old.client.Close()
		if old.sshClient != nil {
			old.sshClient.Close()
		}
		delete(m.clients, cfg.ID)
	}

	var sshClient *gossh.Client
	var dialer func(network, addr string) (net.Conn, error)

	if cfg.SSHEnabled && cfg.SSH != nil {
		sc, err := ssh.NewSSHTunnel(cfg.SSH.Host, cfg.SSH.Port, cfg.SSH.User, cfg.SSH.Password)
		if err != nil {
			return fmt.Errorf("SSH tunnel error: %w", err)
		}
		sshClient = sc
		dialer = ssh.MakeDialer(sc)
	}

	var client redis.UniversalClient

	if cfg.IsCluster {
		addrs := cfg.ClusterAddrs
		if len(addrs) == 0 {
			addrs = []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)}
		}
		opts := &redis.ClusterOptions{
			Addrs:    addrs,
			Password: cfg.Password,
		}
		if dialer != nil {
			opts.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer(network, addr)
			}
		}
		client = redis.NewClusterClient(opts)
	} else {
		opts := &redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password: cfg.Password,
			DB:       cfg.DB,
		}
		if dialer != nil {
			opts.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer(network, addr)
			}
		}
		client = redis.NewClient(opts)
	}

	// 测试连通性
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		if sshClient != nil {
			sshClient.Close()
		}
		return fmt.Errorf("Redis ping failed: %w", err)
	}

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

	opts := conn.client.(*redis.Client).Options()
	newOpts := *opts
	newOpts.DB = db

	// 如果有 SSH dialer，保留
	var dialer func(network, addr string) (net.Conn, error)
	if conn.sshClient != nil {
		dialer = ssh.MakeDialer(conn.sshClient)
	}
	if dialer != nil {
		newOpts.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer(network, addr)
		}
	}

	newClient := redis.NewClient(&newOpts)
	ctx := context.Background()
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
	var sshClient *gossh.Client
	var dialer func(network, addr string) (net.Conn, error)

	if cfg.SSHEnabled && cfg.SSH != nil {
		sc, err := ssh.NewSSHTunnel(cfg.SSH.Host, cfg.SSH.Port, cfg.SSH.User, cfg.SSH.Password)
		if err != nil {
			return fmt.Errorf("SSH tunnel error: %w", err)
		}
		sshClient = sc
		dialer = ssh.MakeDialer(sc)
		defer sc.Close()
	}

	var client redis.UniversalClient
	ctx := context.Background()

	if cfg.IsCluster {
		addrs := cfg.ClusterAddrs
		if len(addrs) == 0 {
			addrs = []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)}
		}
		opts := &redis.ClusterOptions{
			Addrs:    addrs,
			Password: cfg.Password,
		}
		if dialer != nil {
			opts.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer(network, addr)
			}
		}
		client = redis.NewClusterClient(opts)
	} else {
		opts := &redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password: cfg.Password,
			DB:       cfg.DB,
		}
		if dialer != nil {
			opts.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer(network, addr)
			}
		}
		client = redis.NewClient(opts)
	}
	defer client.Close()
	_ = sshClient

	return client.Ping(ctx).Err()
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
