package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"LiteRedis/backend/config"

	gossh "golang.org/x/crypto/ssh"
)

func TestStandaloneSSHAuthenticationClosesWithoutChannels(t *testing.T) {
	setupTestKnownHosts(t)
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	password := "  secret with spaces  "
	serverConfig := &gossh.ServerConfig{PasswordCallback: func(meta gossh.ConnMetadata, pass []byte) (*gossh.Permissions, error) {
		if meta.User() != "tester" || string(pass) != password {
			return nil, fmt.Errorf("bad credentials")
		}
		return nil, nil
	}}
	serverConfig.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	finished := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			finished <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		server, channels, requests, err := gossh.NewServerConn(conn, serverConfig)
		if err != nil {
			finished <- err
			return
		}
		defer server.Close()
		go gossh.DiscardRequests(requests)
		for channel := range channels {
			_ = channel.Reject(gossh.Prohibited, "test must not open channels")
			finished <- fmt.Errorf("unexpected SSH channel")
			return
		}
		finished <- nil
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := TestSSHConnection(config.SSHConfig{Host: "127.0.0.1", Port: port, User: "tester", Password: password}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("SSH test leaked an authenticated connection")
	}
}

func TestBuildAuthMethodsPreservesPassphraseWhitespace(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	passphrase := "  secret with spaces  "
	block, err := gossh.MarshalPrivateKeyWithPassphrase(key, "test", []byte(passphrase))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatal(err)
	}
	methods, err := buildAuthMethods("", path, passphrase)
	if err != nil || len(methods) == 0 {
		t.Fatalf("passphrase whitespace was lost: %d methods, %v", len(methods), err)
	}
}

func TestSSHAddressSupportsIPv6(t *testing.T) {
	if got := sshAddress("2001:db8::2", 22); got != "[2001:db8::2]:22" {
		t.Fatalf("IPv6 SSH address = %q", got)
	}
	if got := sshAddress("127.0.0.1", 2222); got != "127.0.0.1:2222" {
		t.Fatalf("IPv4 SSH address = %q", got)
	}
}
