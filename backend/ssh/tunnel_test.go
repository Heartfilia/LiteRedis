package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

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
