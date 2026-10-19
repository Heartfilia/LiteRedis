package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func setupTestKnownHosts(t *testing.T) {
	t.Helper()
	setKnownHostsPathForTest(filepath.Join(t.TempDir(), "ssh_known_hosts.json"))
	t.Cleanup(func() {
		setKnownHostsPathForTest(filepath.Join(os.TempDir(), "LiteRedis", "ssh_known_hosts.json"))
	})
}

func testPublicKey(t *testing.T) gossh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pub, err := gossh.NewPublicKey(priv.Public())
	if err != nil {
		t.Fatalf("new public key: %v", err)
	}
	return pub
}

func TestTOFURecordsOnFirstUse(t *testing.T) {
	setupTestKnownHosts(t)
	key := testPublicKey(t)

	if err := tofuHostKeyCallback("example.com:22", nil, key); err != nil {
		t.Fatalf("first connect should record and pass, got: %v", err)
	}

	// 再次连接同指纹应通过
	if err := tofuHostKeyCallback("example.com:22", nil, key); err != nil {
		t.Fatalf("second connect with same key should pass, got: %v", err)
	}
}

func TestTOFURejectsChangedKey(t *testing.T) {
	setupTestKnownHosts(t)
	key1 := testPublicKey(t)
	key2 := testPublicKey(t)

	if err := tofuHostKeyCallback("example.com:22", nil, key1); err != nil {
		t.Fatalf("first connect failed: %v", err)
	}
	err := tofuHostKeyCallback("example.com:22", nil, key2)
	if err == nil {
		t.Fatal("changed host key should be rejected")
	}

	// 不同主机互不影响
	if err := tofuHostKeyCallback("other.example.com:22", nil, key2); err != nil {
		t.Fatalf("different host should be recorded independently: %v", err)
	}
}

func TestForgetKnownHost(t *testing.T) {
	setupTestKnownHosts(t)
	key := testPublicKey(t)

	if err := tofuHostKeyCallback("example.com:22", nil, key); err != nil {
		t.Fatalf("first connect failed: %v", err)
	}
	if err := ForgetKnownHost("example.com", 22); err != nil {
		t.Fatalf("forget failed: %v", err)
	}
	// 删除后应重新记录
	if err := tofuHostKeyCallback("example.com:22", nil, key); err != nil {
		t.Fatalf("after forget, connect should re-record: %v", err)
	}
	if err := ForgetKnownHost("example.com", 22); err != nil {
		t.Fatalf("second forget failed: %v", err)
	}
	if err := ForgetKnownHost("example.com", 22); err == nil {
		t.Fatal("forgetting a non-existent host should fail")
	}
}

func TestTOFUCorruptedFileResets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh_known_hosts.json")
	// 写入损坏内容
	if err := os.WriteFile(path, []byte("not json at all"), 0600); err != nil {
		t.Fatal(err)
	}
	setKnownHostsPathForTest(path)
	t.Cleanup(func() {
		setKnownHostsPathForTest(filepath.Join(os.TempDir(), "LiteRedis", "ssh_known_hosts.json"))
	})

	key := testPublicKey(t)
	if err := tofuHostKeyCallback("example.com:22", nil, key); err != nil {
		t.Fatalf("corrupted file should reset and re-record, got: %v", err)
	}
}
