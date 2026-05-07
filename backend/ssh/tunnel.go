package ssh

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"LiteRedis/backend/config"
	gossh "golang.org/x/crypto/ssh"
)

const defaultSSHTimeout = 10 * time.Second
const defaultSSHDialTimeout = 10 * time.Second
const minSupportedDeadline = 100 * time.Millisecond

// NewSSHTunnel 创建 SSH 客户端，返回可用于 Dial 的客户端
func NewSSHTunnel(host string, port int, user string, password string) (*gossh.Client, error) {
	return NewSSHTunnelWithConfig(host, port, user, password, "", "", defaultSSHTimeout)
}

// NewSSHTunnelWithTimeout 创建带超时限制的 SSH 客户端。
// 先自己用 net.DialTimeout 建立 TCP（超时更可靠），再进行 SSH 握手。
func NewSSHTunnelWithTimeout(host string, port int, user string, password string, timeout time.Duration) (*gossh.Client, error) {
	return NewSSHTunnelWithConfig(host, port, user, password, "", "", timeout)
}

func NewSSHTunnelWithConfig(host string, port int, user string, password string, privateKeyPath string, passphrase string, timeout time.Duration) (*gossh.Client, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("SSH host is required")
	}
	if timeout <= 0 {
		timeout = defaultSSHTimeout
	}
	config.AppendDebugLog("[ssh] begin host=%s port=%d user=%s timeout=%s keyPath=%q", host, port, user, timeout, privateKeyPath)
	authMethods, err := buildAuthMethods(password, privateKeyPath, passphrase)
	if err != nil {
		config.AppendDebugLog("[ssh] build auth failed: %v", err)
		return nil, err
	}
	if len(authMethods) == 0 {
		config.AppendDebugLog("[ssh] no auth methods available")
		return nil, fmt.Errorf("no SSH auth methods available; provide password or private key")
	}

	clientConfig := &gossh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	// 自己控制 TCP 连接超时，避免某些 Windows 环境下 net.DialTimeout 不生效的问题
	tcpConn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		config.AppendDebugLog("[ssh] tcp dial failed addr=%s err=%v", addr, err)
		return nil, fmt.Errorf("SSH tcp dial failed: %w", err)
	}
	config.AppendDebugLog("[ssh] tcp dial ok addr=%s", addr)
	if err := tcpConn.SetDeadline(time.Now().Add(timeout)); err != nil {
		tcpConn.Close()
		config.AppendDebugLog("[ssh] set deadline failed addr=%s err=%v", addr, err)
		return nil, fmt.Errorf("SSH set deadline failed: %w", err)
	}

	// SSH 握手
	conn, chans, reqs, err := gossh.NewClientConn(tcpConn, addr, clientConfig)
	if err != nil {
		tcpConn.Close()
		config.AppendDebugLog("[ssh] handshake failed addr=%s err=%v", addr, err)
		return nil, fmt.Errorf("SSH handshake failed: %w", err)
	}
	_ = tcpConn.SetDeadline(time.Time{})
	config.AppendDebugLog("[ssh] handshake ok addr=%s", addr)

	client := gossh.NewClient(conn, chans, reqs)
	return client, nil
}

func buildAuthMethods(password string, privateKeyPath string, passphrase string) ([]gossh.AuthMethod, error) {
	authMethods := make([]gossh.AuthMethod, 0, 4)
	password = strings.TrimSpace(password)
	privateKeyPath = strings.TrimSpace(privateKeyPath)
	passphrase = strings.TrimSpace(passphrase)

	if password != "" {
		config.AppendDebugLog("[ssh] add password auth")
		authMethods = append(authMethods,
			gossh.Password(password),
			gossh.KeyboardInteractive(func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = password
				}
				return answers, nil
			}),
		)
	}

	keyPaths := candidatePrivateKeyPaths(privateKeyPath)
	var firstKeyErr error
	for _, path := range keyPaths {
		signer, err := loadPrivateKeySigner(path, passphrase)
		if err != nil {
			config.AppendDebugLog("[ssh] load private key failed path=%s err=%v", path, err)
			if firstKeyErr == nil {
				firstKeyErr = fmt.Errorf("%s: %w", path, err)
			}
			continue
		}
		config.AppendDebugLog("[ssh] add public key auth path=%s", path)
		authMethods = append(authMethods, gossh.PublicKeys(signer))
	}

	if len(authMethods) == 0 && firstKeyErr != nil {
		return nil, fmt.Errorf("failed to load SSH auth: %w", firstKeyErr)
	}
	return authMethods, nil
}

func loadPrivateKeySigner(path string, passphrase string) (gossh.Signer, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if passphrase != "" {
		if signer, err := gossh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(passphrase)); err == nil {
			return signer, nil
		}
	}
	return gossh.ParsePrivateKey(pemBytes)
}

func candidatePrivateKeyPaths(explicitPath string) []string {
	paths := make([]string, 0, 8)
	seen := map[string]struct{}{}
	addPath := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
	}

	addPath(explicitPath)

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		home = os.Getenv("USERPROFILE")
	}
	if strings.TrimSpace(home) == "" {
		return paths
	}

	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa", "id_dsa"} {
		addPath(filepath.Join(home, ".ssh", name))
	}
	return paths
}

// deadlineConn 包装 SSH channel 连接，将不支持的 deadline 方法变为 no-op
// go-redis 内部会调用 SetDeadline/SetReadDeadline/SetWriteDeadline，
// 而 SSH channel 不实现这些接口，会返回 "deadline not supported" 错误。
type deadlineConn struct {
	net.Conn
	mu         sync.Mutex
	readTimer  *time.Timer
	writeTimer *time.Timer
	bothTimer  *time.Timer
}

func (c *deadlineConn) stopTimer(timer **time.Timer) {
	if *timer == nil {
		return
	}
	(*timer).Stop()
	*timer = nil
}

func (c *deadlineConn) resetTimer(timer **time.Timer, deadline time.Time, label string) {
	c.stopTimer(timer)
	if deadline.IsZero() {
		return
	}
	d := time.Until(deadline)
	if d <= 0 {
		config.AppendDebugLog("[ssh] %s deadline reached immediately, closing conn", label)
		_ = c.Conn.Close()
		return
	}
	if d < minSupportedDeadline {
		config.AppendDebugLog("[ssh] %s deadline too small (%s), clamp to %s", label, d, minSupportedDeadline)
		d = minSupportedDeadline
	}
	*timer = time.AfterFunc(d, func() {
		config.AppendDebugLog("[ssh] %s deadline reached after %s, closing conn", label, d)
		_ = c.Conn.Close()
	})
}

func (c *deadlineConn) SetDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetTimer(&c.bothTimer, deadline, "read/write")
	return nil
}

func (c *deadlineConn) SetReadDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetTimer(&c.readTimer, deadline, "read")
	return nil
}

func (c *deadlineConn) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetTimer(&c.writeTimer, deadline, "write")
	return nil
}

func (c *deadlineConn) Close() error {
	c.mu.Lock()
	c.stopTimer(&c.readTimer)
	c.stopTimer(&c.writeTimer)
	c.stopTimer(&c.bothTimer)
	c.mu.Unlock()
	return c.Conn.Close()
}

// MakeDialer 返回一个使用 SSH 客户端拨号的 Dialer 函数
// 如果 sshClient 为 nil，则走正常 TCP
// timeout 控制单次 dial 的最大等待时间，防止 sshClient.Dial 永久阻塞
func MakeDialer(sshClient *gossh.Client, timeout time.Duration) func(network, addr string) (net.Conn, error) {
	if sshClient == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = defaultSSHDialTimeout
	}
	return func(network, addr string) (net.Conn, error) {
		type result struct {
			conn net.Conn
			err  error
		}
		done := make(chan result, 1)
		go func() {
			config.AppendDebugLog("[ssh] tunnel dial begin network=%s addr=%s timeout=%s", network, addr, timeout)
			conn, err := sshClient.Dial(network, addr)
			if err != nil {
				config.AppendDebugLog("[ssh] tunnel dial failed addr=%s err=%v", addr, err)
				done <- result{nil, err}
				return
			}
			config.AppendDebugLog("[ssh] tunnel dial ok addr=%s", addr)
			done <- result{&deadlineConn{Conn: conn}, nil}
		}()
		select {
		case r := <-done:
			return r.conn, r.err
		case <-time.After(timeout):
			config.AppendDebugLog("[ssh] tunnel dial timeout addr=%s timeout=%s", addr, timeout)
			return nil, fmt.Errorf("ssh dial %s timeout after %v", addr, timeout)
		}
	}
}

// MakeContextDialer 返回一个符合 go-redis Dialer 签名的函数（带 context）
// 会同时尊重 ctx 取消和 timeout 限制
func MakeContextDialer(sshClient *gossh.Client, timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := MakeDialer(sshClient, timeout)
	if dialer == nil {
		return nil
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		type result struct {
			conn net.Conn
			err  error
		}
		done := make(chan result, 1)
		go func() {
			conn, err := dialer(network, addr)
			done <- result{conn, err}
		}()
		select {
		case r := <-done:
			return r.conn, r.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
