package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "v0.4.3-beta.1", right: "v0.4.3-beta.2", want: -1},
		{left: "0.4.3-beta.2", right: "0.4.3", want: -1},
		{left: "0.4.3", right: "0.4.3-beta.2", want: 1},
		{left: "0.4.3+build.1", right: "0.4.3+build.2", want: 0},
		{left: "dev", right: "0.4.3", want: -1},
	}
	for _, test := range tests {
		if got := compareSemver(test.left, test.right); got != test.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestSelectLatestReleaseRespectsChannels(t *testing.T) {
	releases := []ReleaseInfo{
		{TagName: "v0.4.4-beta.2", Prerelease: true},
		{TagName: "v0.4.3"},
		{TagName: "v0.5.0", Draft: true},
		{TagName: "not-semver"},
	}

	stable, ok := selectLatestRelease(releases, false)
	if !ok || stable.TagName != "v0.4.3" {
		t.Fatalf("stable selection = %#v, ok=%v", stable, ok)
	}
	prerelease, ok := selectLatestRelease(releases, true)
	if !ok || prerelease.TagName != "v0.4.4-beta.2" {
		t.Fatalf("prerelease selection = %#v, ok=%v", prerelease, ok)
	}
}

func TestIsPrereleaseVersion(t *testing.T) {
	if !isPrereleaseVersion("v0.4.3-beta.1") {
		t.Fatal("beta version was not recognized as prerelease")
	}
	if isPrereleaseVersion("v0.4.3") || isPrereleaseVersion("dev") {
		t.Fatal("stable or invalid version was recognized as prerelease")
	}
}

func TestValidateDownloadedAsset(t *testing.T) {
	content := []byte("verified installer")
	path := filepath.Join(t.TempDir(), "LiteRedis-installer.exe")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	manifest := []byte(fmt.Sprintf("%x  release/LiteRedis-installer.exe\n", digest))
	asset := ReleaseAsset{Name: "LiteRedis-installer.exe", Size: int64(len(content))}

	if err := validateDownloadedAsset(path, asset, manifest); err != nil {
		t.Fatalf("valid asset rejected: %v", err)
	}
	if err := validateDownloadedAsset(path, asset, []byte(fmt.Sprintf("%064x  LiteRedis-installer.exe\n", 1))); err == nil {
		t.Fatal("invalid digest was accepted")
	}
	asset.Size++
	if err := validateDownloadedAsset(path, asset, manifest); err == nil {
		t.Fatal("invalid size was accepted")
	}
}

func TestChecksumForAssetRejectsMissingOrMalformedEntry(t *testing.T) {
	if _, err := checksumForAsset([]byte("abcd  other.exe\n"), "target.exe"); err == nil {
		t.Fatal("missing checksum entry was accepted")
	}
	if _, err := checksumForAsset([]byte("not-a-digest  target.exe\n"), "target.exe"); err == nil {
		t.Fatal("malformed checksum entry was accepted")
	}
}

func TestPickAssetForPlatformMatchesArchitecture(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "LiteRedis-macos-arm64.dmg"},
		{Name: "LiteRedis-macos-amd64.dmg"},
		{Name: "LiteRedis-linux-amd64.tar.gz"},
		{Name: "LiteRedis-arm64-installer.exe"},
		{Name: "LiteRedis-amd64-installer.exe"},
	}
	if got := pickAssetForPlatform(assets, "darwin", "arm64"); got == nil || got.Name != "LiteRedis-macos-arm64.dmg" {
		t.Fatalf("darwin/arm64 asset = %#v", got)
	}
	if got := pickAssetForPlatform(assets, "darwin", "amd64"); got == nil || got.Name != "LiteRedis-macos-amd64.dmg" {
		t.Fatalf("darwin/amd64 asset = %#v", got)
	}
	if got := pickAssetForPlatform(assets[:1], "darwin", "amd64"); got != nil {
		t.Fatalf("wrong-architecture asset selected: %#v", got)
	}
	if got := pickAssetForPlatform(assets, "windows", "amd64"); got == nil || got.Name != "LiteRedis-amd64-installer.exe" {
		t.Fatalf("windows/amd64 asset = %#v", got)
	}
}

func TestDownloadFileWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	err := downloadFileWithTimeout(context.Background(), server.URL, filepath.Join(t.TempDir(), "asset"), 20*time.Millisecond)
	if err == nil {
		t.Fatal("stalled download did not time out")
	}
}

func TestCleanupStaleUpdateDirs(t *testing.T) {
	root := t.TempDir()
	oldCurrent := filepath.Join(root, updateTempPrefix+"old")
	oldLegacy := filepath.Join(root, "litetredis-update-old")
	recent := filepath.Join(root, updateTempPrefix+"recent")
	unrelated := filepath.Join(root, "other-old")
	for _, path := range []string{oldCurrent, oldLegacy, recent, unrelated} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	oldTime := now.Add(-2 * time.Hour)
	for _, path := range []string{oldCurrent, oldLegacy, unrelated} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}

	if err := cleanupStaleUpdateDirs(root, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{oldCurrent, oldLegacy} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stale update directory still exists: %s", path)
		}
	}
	for _, path := range []string{recent, unrelated} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("directory was removed unexpectedly: %s: %v", path, err)
		}
	}
}

func TestScheduleUpdateCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), updateTempPrefix+"scheduled")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	scheduleUpdateCleanup(path, 10*time.Millisecond)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scheduled update directory was not removed")
}
