package ssh

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"LiteRedis/backend/config"

	gossh "golang.org/x/crypto/ssh"
)

// knownHostEntry 单条 SSH 主机指纹记录
type knownHostEntry struct {
	Fingerprint string `json:"fingerprint"`
	Algorithm   string `json:"algorithm"`
	AddedAt     string `json:"added_at"`
}

// knownHostStore 指纹文件结构
type knownHostStore struct {
	Hosts map[string]knownHostEntry `json:"hosts"`
}

var (
	knownHostsMu   sync.Mutex
	knownHostsPath string
)

func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	appDir := filepath.Join(dir, "LiteRedis")
	_ = os.MkdirAll(appDir, 0700)
	knownHostsPath = filepath.Join(appDir, "ssh_known_hosts.json")
}

// setKnownHostsPathForTest 供测试注入临时路径
func setKnownHostsPathForTest(path string) {
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()
	knownHostsPath = path
}

// tofuHostKeyCallback 实现 Trust-On-First-Use 主机密钥校验。
// 首次连接自动记录指纹；之后每次连接比对指纹，不匹配则拒绝（可能存在中间人攻击）。
func tofuHostKeyCallback(hostname string, remote net.Addr, key gossh.PublicKey) error {
	fingerprint := gossh.FingerprintSHA256(key)
	algorithm := key.Type()

	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()

	store, err := loadKnownHostsLocked()
	if err != nil {
		return fmt.Errorf("SSH host key check failed: %w", err)
	}
	if entry, ok := store.Hosts[hostname]; ok {
		if entry.Fingerprint == fingerprint {
			return nil
		}
		return fmt.Errorf(
			"SSH host key mismatch for %s: recorded %s (%s), server presented %s (%s). "+
				"This may indicate a man-in-the-middle attack, or the server was rebuilt/rekeyed. "+
				"If you trust the new key, reset the recorded fingerprint via the connection form",
			hostname, entry.Fingerprint, entry.Algorithm, fingerprint, algorithm)
	}
	if store.Hosts == nil {
		store.Hosts = make(map[string]knownHostEntry)
	}
	store.Hosts[hostname] = knownHostEntry{
		Fingerprint: fingerprint,
		Algorithm:   algorithm,
		AddedAt:     time.Now().Format(time.RFC3339),
	}
	if err := saveKnownHostsLocked(store); err != nil {
		return fmt.Errorf("SSH host key check failed: %w", err)
	}
	config.AppendDebugLog("[ssh] tofu recorded host key host=%s alg=%s fp=%s", hostname, algorithm, fingerprint)
	return nil
}

// ForgetKnownHost 删除指定主机的已记录指纹（服务器换钥后用户手动重置用）
func ForgetKnownHost(host string, port int) error {
	if host == "" {
		return fmt.Errorf("SSH host is required")
	}
	hostname := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()

	store, err := loadKnownHostsLocked()
	if err != nil {
		return err
	}
	if _, ok := store.Hosts[hostname]; !ok {
		return fmt.Errorf("no recorded fingerprint for %s", hostname)
	}
	delete(store.Hosts, hostname)
	return saveKnownHostsLocked(store)
}

func loadKnownHostsLocked() (knownHostStore, error) {
	store := knownHostStore{Hosts: make(map[string]knownHostEntry)}
	data, err := os.ReadFile(knownHostsPath)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return store, err
	}
	if err := json.Unmarshal(data, &store); err != nil {
		// 指纹文件损坏时视为空存储，重新开始 TOFU 学习
		config.AppendDebugLog("[ssh] known hosts file corrupted, resetting: %v", err)
		return knownHostStore{Hosts: make(map[string]knownHostEntry)}, nil
	}
	if store.Hosts == nil {
		store.Hosts = make(map[string]knownHostEntry)
	}
	return store, nil
}

func saveKnownHostsLocked(store knownHostStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return config.AtomicWriteFile(knownHostsPath, data, 0600)
}
