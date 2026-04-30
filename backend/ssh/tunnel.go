package ssh

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

const defaultSSHTimeout = 10 * time.Second
const defaultSSHDialTimeout = 10 * time.Second

// NewSSHTunnel 创建 SSH 客户端，返回可用于 Dial 的客户端
func NewSSHTunnel(host string, port int, user string, password string) (*gossh.Client, error) {
	return NewSSHTunnelWithTimeout(host, port, user, password, defaultSSHTimeout)
}

// NewSSHTunnelWithTimeout 创建带超时限制的 SSH 客户端。
// 先自己用 net.DialTimeout 建立 TCP（超时更可靠），再进行 SSH 握手。
func NewSSHTunnelWithTimeout(host string, port int, user string, password string, timeout time.Duration) (*gossh.Client, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("SSH host is required")
	}
	if timeout <= 0 {
		timeout = defaultSSHTimeout
	}

	config := &gossh.ClientConfig{
		User: user,
		Auth: []gossh.AuthMethod{
			gossh.Password(password),
			gossh.KeyboardInteractive(func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	// 自己控制 TCP 连接超时，避免某些 Windows 环境下 net.DialTimeout 不生效的问题
	tcpConn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("SSH tcp dial failed: %w", err)
	}

	// SSH 握手
	conn, chans, reqs, err := gossh.NewClientConn(tcpConn, addr, config)
	if err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("SSH handshake failed: %w", err)
	}

	client := gossh.NewClient(conn, chans, reqs)
	return client, nil
}

// deadlineConn 包装 SSH channel 连接，将不支持的 deadline 方法变为 no-op
// go-redis 内部会调用 SetDeadline/SetReadDeadline/SetWriteDeadline，
// 而 SSH channel 不实现这些接口，会返回 "deadline not supported" 错误。
type deadlineConn struct {
	net.Conn
}

func (c *deadlineConn) SetDeadline(_ time.Time) error      { return nil }
func (c *deadlineConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *deadlineConn) SetWriteDeadline(_ time.Time) error { return nil }

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
			conn, err := sshClient.Dial(network, addr)
			if err != nil {
				done <- result{nil, err}
				return
			}
			done <- result{&deadlineConn{conn}, nil}
		}()
		select {
		case r := <-done:
			return r.conn, r.err
		case <-time.After(timeout):
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
