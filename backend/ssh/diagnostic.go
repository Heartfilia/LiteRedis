package ssh

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"LiteRedis/backend/config"
	gossh "golang.org/x/crypto/ssh"
)

var ErrDiagnosticUntrusted = errors.New("ssh_untrusted")
var ErrDiagnosticChanged = errors.New("ssh_changed")
var ErrDiagnosticTrust = errors.New("ssh_trust_unavailable")

// diagnosticHostKeyCallback is strictly read-only. Unknown hosts must first be
// approved by the ordinary connection flow, never silently trusted by diagnosis.
func diagnosticHostKeyCallback(host string, _ net.Addr, key gossh.PublicKey) error {
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()
	store, err := loadKnownHostsLocked()
	if err != nil {
		return ErrDiagnosticTrust
	}
	entry, ok := store.Hosts[host]
	if !ok {
		return ErrDiagnosticUntrusted
	}
	if entry.Fingerprint != gossh.FingerprintSHA256(key) {
		return ErrDiagnosticChanged
	}
	return nil
}

// NewDiagnosticClient uses only explicit credentials and existing trust records.
// It neither scans default private keys nor records/updates host fingerprints.
func NewDiagnosticClient(parent context.Context, cfg config.SSHConfig) (*gossh.Client, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	var auth []gossh.AuthMethod
	if cfg.Password != "" {
		auth = append(auth, gossh.Password(cfg.Password), gossh.KeyboardInteractive(func(_ string, _ string, q []string, _ []bool) ([]string, error) {
			a := make([]string, len(q))
			for i := range a {
				a[i] = cfg.Password
			}
			return a, nil
		}))
	}
	if strings.TrimSpace(cfg.PrivateKeyPath) != "" {
		signer, err := loadPrivateKeySigner(cfg.PrivateKeyPath, cfg.Passphrase)
		if err != nil {
			cancel()
			return nil, errors.New("ssh_key_invalid")
		}
		auth = append(auth, gossh.PublicKeys(signer))
	}
	if strings.TrimSpace(cfg.Host) == "" || cfg.Port < 1 || cfg.Port > 65535 || strings.TrimSpace(cfg.User) == "" || len(auth) == 0 {
		cancel()
		return nil, errors.New("ssh_config_invalid")
	}
	addr := sshAddress(cfg.Host, cfg.Port)
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		cancel()
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	cleanup := func() { stop(); raw.Close(); cancel() }
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	conn, chans, reqs, err := gossh.NewClientConn(raw, addr, &gossh.ClientConfig{User: strings.TrimSpace(cfg.User), Auth: auth, HostKeyCallback: diagnosticHostKeyCallback})
	if err != nil {
		cleanup()
		return nil, err
	}
	_ = raw.SetDeadline(time.Time{})
	client := gossh.NewClient(conn, chans, reqs)
	// Keep the bounded context alive for channels opened after authentication.
	go func() { _ = client.Wait(); stop(); cancel() }()
	return client, nil
}
