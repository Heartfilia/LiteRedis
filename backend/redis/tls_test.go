package redis

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"LiteRedis/backend/config"
)

func writeTestCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "LiteRedis Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildTLSConfig(t *testing.T) {
	if cfg, err := buildTLSConfig(config.ConnectionConfig{}); cfg != nil || err != nil {
		t.Fatalf("disabled TLS = %v, %v", cfg, err)
	}

	cfg, err := buildTLSConfig(config.ConnectionConfig{TLSEnabled: true, Host: "redis.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerName != "redis.example.com" || cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.InsecureSkipVerify {
		t.Fatal("skip verify should be off by default")
	}

	skip, err := buildTLSConfig(config.ConnectionConfig{TLSEnabled: true, TLSSkipVerify: true})
	if err != nil || !skip.InsecureSkipVerify {
		t.Fatalf("skip verify = %#v, %v", skip, err)
	}

	if _, err := buildTLSConfig(config.ConnectionConfig{TLSEnabled: true, TLSCAFile: "/nonexistent/ca.pem"}); err == nil {
		t.Fatal("missing CA file was accepted")
	}

	invalid := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(invalid, []byte("not a pem"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildTLSConfig(config.ConnectionConfig{TLSEnabled: true, TLSCAFile: invalid}); err == nil {
		t.Fatal("invalid CA file was accepted")
	}

	caPath := writeTestCA(t)
	withCA, err := buildTLSConfig(config.ConnectionConfig{TLSEnabled: true, TLSCAFile: caPath})
	if err != nil {
		t.Fatal(err)
	}
	if withCA.RootCAs == nil {
		t.Fatal("RootCAs was not set")
	}
}

func TestBuildOptionsCarryUsernameAndTLS(t *testing.T) {
	cfg := config.ConnectionConfig{Username: "app", Password: "secret", TLSEnabled: true}
	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	single := buildRedisOptions("127.0.0.1:6379", cfg, 2, nil, tlsConfig)
	if single.Username != "app" || single.Password != "secret" || single.DB != 2 || single.TLSConfig == nil {
		t.Fatalf("single options = %#v", single)
	}

	cluster := buildClusterOptions([]string{"127.0.0.1:7000"}, cfg, nil, tlsConfig)
	if cluster.Username != "app" || cluster.Password != "secret" || cluster.TLSConfig == nil {
		t.Fatalf("cluster options = %#v", cluster)
	}
}

func TestSameConnectionTransportDetectsTLSAndUsernameChanges(t *testing.T) {
	base := config.ConnectionConfig{Host: "127.0.0.1", Port: 6379}

	tlsOn := base
	tlsOn.TLSEnabled = true
	if sameConnectionTransport(base, tlsOn) {
		t.Fatal("TLS toggle was ignored")
	}

	skipVerify := base
	skipVerify.TLSSkipVerify = true
	if sameConnectionTransport(base, skipVerify) {
		t.Fatal("TLS skip verify change was ignored")
	}

	caFile := base
	caFile.TLSCAFile = "/tmp/ca.pem"
	if sameConnectionTransport(base, caFile) {
		t.Fatal("CA file change was ignored")
	}

	withUser := base
	withUser.Username = "app"
	if sameConnectionTransport(base, withUser) {
		t.Fatal("username change was ignored")
	}
}
