package ssh

import (
	"fmt"
	"net"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

// NewSSHTunnel 创建 SSH 客户端，返回可用于 Dial 的客户端
func NewSSHTunnel(host string, port int, user string, password string) (*gossh.Client, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("SSH host is required")
	}

	config := &gossh.ClientConfig{
		User: user,
		Auth: []gossh.AuthMethod{
			gossh.Password(password),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := gossh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH dial failed: %w", err)
	}
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
func MakeDialer(sshClient *gossh.Client) func(network, addr string) (net.Conn, error) {
	if sshClient == nil {
		return nil
	}
	return func(network, addr string) (net.Conn, error) {
		conn, err := sshClient.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		return &deadlineConn{conn}, nil
	}
}
