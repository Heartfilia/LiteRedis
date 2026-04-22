package ssh

import (
	"fmt"
	"net"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

// NewSSHTunnel 创建 SSH 客户端，返回可用于 Dial 的客户端
func NewSSHTunnel(host string, port int, user string, password string) (*gossh.Client, error) {
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

// MakeDialer 返回一个使用 SSH 客户端拨号的 Dialer 函数
// 如果 sshClient 为 nil，则走正常 TCP
func MakeDialer(sshClient *gossh.Client) func(network, addr string) (net.Conn, error) {
	if sshClient == nil {
		return nil
	}
	return func(network, addr string) (net.Conn, error) {
		return sshClient.Dial(network, addr)
	}
}
