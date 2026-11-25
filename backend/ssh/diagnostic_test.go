package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"LiteRedis/backend/config"
	gossh "golang.org/x/crypto/ssh"
)

func TestDiagnosticTrustReadOnly(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	knownHostsMu.Lock()
	old := knownHostsPath
	knownHostsMu.Unlock()
	path := filepath.Join(t.TempDir(), "known.json")
	setKnownHostsPathForTest(path)
	t.Cleanup(func() { setKnownHostsPathForTest(old) })
	host := "diagnostic.test:22"
	if err := diagnosticHostKeyCallback(host, nil, signer.PublicKey()); !errors.Is(err, ErrDiagnosticUntrusted) {
		t.Fatalf("unknown=%v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unknown host wrote trust")
	}
	store := knownHostStore{Hosts: map[string]knownHostEntry{host: {Fingerprint: gossh.FingerprintSHA256(signer.PublicKey())}}}
	data, _ := json.Marshal(store)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := diagnosticHostKeyCallback(host, nil, signer.PublicKey()); err != nil {
		t.Fatal(err)
	}
	store.Hosts[host] = knownHostEntry{Fingerprint: "different"}
	changed, _ := json.Marshal(store)
	os.WriteFile(path, changed, 0600)
	if err := diagnosticHostKeyCallback(host, nil, signer.PublicKey()); !errors.Is(err, ErrDiagnosticChanged) {
		t.Fatalf("changed=%v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(changed) {
		t.Fatal("changed host overwrote trust")
	}
	os.WriteFile(path, []byte("bad json"), 0600)
	if err := diagnosticHostKeyCallback(host, nil, signer.PublicKey()); !errors.Is(err, ErrDiagnosticTrust) {
		t.Fatalf("corrupt trust=%v", err)
	}
}

func TestDiagnosticSSHTrustedAuthenticationAndCancellation(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	knownHostsMu.Lock()
	old := knownHostsPath
	knownHostsMu.Unlock()
	path := filepath.Join(t.TempDir(), "known.json")
	setKnownHostsPathForTest(path)
	t.Cleanup(func() { setKnownHostsPathForTest(old) })
	data, _ := json.Marshal(knownHostStore{Hosts: map[string]knownHostEntry{listener.Addr().String(): {Fingerprint: gossh.FingerprintSHA256(signer.PublicKey())}}})
	os.WriteFile(path, data, 0600)
	done := make(chan struct{})
	go func() {
		defer close(done)
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		serverCfg := &gossh.ServerConfig{PasswordCallback: func(_ gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			if string(password) != "secret" {
				return nil, errors.New("denied")
			}
			return nil, nil
		}}
		serverCfg.AddHostKey(signer)
		conn, chans, reqs, err := gossh.NewServerConn(raw, serverCfg)
		if err != nil {
			return
		}
		defer conn.Close()
		go gossh.DiscardRequests(reqs)
		for ch := range chans {
			ch.Reject(gossh.UnknownChannelType, "unsupported")
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	n, _ := strconv.Atoi(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := NewDiagnosticClient(ctx, config.SSHConfig{Host: host, Port: n, User: "test", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSH cancellation left socket open")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) {
		t.Fatal("diagnosis changed trusted fingerprint")
	}
}
