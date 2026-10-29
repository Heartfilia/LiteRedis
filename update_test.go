package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"LiteRedis/backend/config"
	redisbackend "LiteRedis/backend/redis"
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

func TestPickAssetPrefersPortableWindowsExe(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "LiteRedis-amd64-installer.exe"},
		{Name: "LiteRedis.exe"},
	}
	got := pickAssetForPlatform(assets, "windows", "amd64")
	if got == nil || got.Name != "LiteRedis.exe" {
		t.Fatalf("portable windows asset = %#v", got)
	}
	if got := pickAssetForPlatform([]ReleaseAsset{{Name: "LiteRedis-amd64-installer.exe"}}, "windows", "amd64"); got == nil {
		t.Fatal("installer fallback was not selected")
	}
}

func TestExtractTarGzBinary(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "pkg.tar.gz")
	payload := []byte("new binary")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "LiteRedis", Mode: 0755, Size: int64(len(payload)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "LiteRedis.new")
	if err := extractTarGzBinary(archivePath, "LiteRedis", target); err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("extracted payload = %q", got)
	}
	if err := extractTarGzBinary(archivePath, "Missing", filepath.Join(t.TempDir(), "x")); err == nil {
		t.Fatal("missing member was accepted")
	}
}

func TestPercentOf(t *testing.T) {
	if got := percentOf(5, 10); got != 50 {
		t.Fatalf("percentOf(5,10) = %v", got)
	}
	if got := percentOf(20, 10); got != 100 {
		t.Fatalf("percentOf overflow = %v", got)
	}
	if got := percentOf(1, 0); got != 0 {
		t.Fatalf("percentOf unknown total = %v", got)
	}
}

func TestDownloadFileWithProgress(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 300*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "asset.bin")
	var updates []UpdateProgress
	err := downloadFileWithProgress(context.Background(), server.URL, target, 0, func(p UpdateProgress) {
		updates = append(updates, p)
	})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(payload)) {
		t.Fatalf("downloaded size = %d", info.Size())
	}
	if len(updates) == 0 {
		t.Fatal("no progress reported")
	}
	last := updates[len(updates)-1]
	if last.Percent != 100 || last.Received != int64(len(payload)) || last.Total != int64(len(payload)) {
		t.Fatalf("final progress = %#v", last)
	}
}

func TestFetchGitHubReleaseJSONCachesResponses(t *testing.T) {
	resetReleaseCacheForTest()
	defer resetReleaseCacheForTest()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[{"name":"LiteRedis-macos-arm64.dmg"}]}`))
	}))
	defer server.Close()

	var first ReleaseInfo
	if err := fetchGitHubReleaseJSON(context.Background(), server.URL, &first); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if first.TagName != "v9.9.9" {
		t.Fatalf("tag = %q", first.TagName)
	}

	// 缓存期内不应该再打网络
	var second ReleaseInfo
	if err := fetchGitHubReleaseJSON(context.Background(), server.URL, &second); err != nil {
		t.Fatalf("cached fetch: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("cache was not used, calls = %d", calls.Load())
	}

	// 过期后用 If-None-Match 重新校验，304 也要返回缓存内容
	expireReleaseCacheForTest(releaseCacheKey(server.URL))
	var third ReleaseInfo
	if err := fetchGitHubReleaseJSON(context.Background(), server.URL, &third); err != nil {
		t.Fatalf("revalidate fetch: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("revalidation did not hit network, calls = %d", calls.Load())
	}
	if third.TagName != "v9.9.9" {
		t.Fatalf("304 body = %q", third.TagName)
	}
}

func TestFetchGitHubReleaseJSONRateLimitedMessage(t *testing.T) {
	resetReleaseCacheForTest()
	defer resetReleaseCacheForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	var out ReleaseInfo
	err := fetchGitHubReleaseJSON(context.Background(), server.URL, &out)
	if err == nil || !strings.Contains(err.Error(), "IP 访问限制") {
		t.Fatalf("rate limit error = %v", err)
	}
	if _, ok := loadReleaseCache(releaseCacheKey(server.URL)); ok {
		t.Fatal("failed response should not be cached")
	}
}

// 403 但没有配额耗尽标记：说明是代理/出口 IP 被拦，不能误报成配额问题
func TestFetchGitHubReleaseJSONForbiddenWithoutRateLimit(t *testing.T) {
	resetReleaseCacheForTest()
	defer resetReleaseCacheForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`blocked by upstream proxy`))
	}))
	defer server.Close()

	var out ReleaseInfo
	err := fetchGitHubReleaseJSON(context.Background(), server.URL, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	message := err.Error()
	if strings.Contains(message, "IP 访问限制") {
		t.Fatalf("403 without rate-limit headers was reported as quota: %s", message)
	}
	if !strings.Contains(message, "403") || !strings.Contains(message, "blocked by upstream proxy") {
		t.Fatalf("error lacks diagnosis detail: %s", message)
	}
}

func TestBuildUpdateHTTPClientProxyModes(t *testing.T) {
	cases := []struct {
		name      string
		settings  config.AppSettings
		wantErr   bool
		wantProxy bool
	}{
		{name: "未开启代理", settings: config.AppSettings{}, wantErr: false, wantProxy: false},
		{name: "开启但地址为空", settings: config.AppSettings{ProxyEnabled: true}, wantErr: true},
		{name: "地址非法", settings: config.AppSettings{ProxyEnabled: true, ProxyURL: "://bad"}, wantErr: true},
		{name: "http 代理", settings: config.AppSettings{ProxyEnabled: true, ProxyURL: "http://127.0.0.1:7890"}, wantProxy: true},
		{name: "socks5 代理", settings: config.AppSettings{ProxyEnabled: true, ProxyURL: "socks5://127.0.0.1:1080"}, wantProxy: true},
	}
	for _, test := range cases {
		client, err := buildUpdateHTTPClient(test.settings, 5*time.Second)
		if test.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", test.name, err)
		}
		hasProxy := client.Transport != nil
		if hasProxy != test.wantProxy {
			t.Fatalf("%s: transport set = %v, want %v", test.name, hasProxy, test.wantProxy)
		}
	}
}

func TestUpdateProxyHintRedactsCredentials(t *testing.T) {
	settings := config.AppSettings{ProxyEnabled: true, ProxyURL: "http://user:secret@127.0.0.1:7890"}
	if !settings.ProxyEnabled {
		t.Fatal("sanity")
	}
	redacted := redisbackend.RedactProxyURL(settings.ProxyURL)
	if strings.Contains(redacted, "secret") {
		t.Fatalf("password leaked: %s", redacted)
	}
}

func TestReleaseNotesAPIURL(t *testing.T) {
	const prefix = "https://api.github.com/repos/Heartfilia/LiteRedis/releases/tags/v"
	cases := []struct {
		in   string
		want string
	}{
		{"1.2.3", prefix + "1.2.3"},
		{"v1.2.3", prefix + "1.2.3"},
		{"V1.2.3", prefix + "1.2.3"},
		{"  v1.2.3  ", prefix + "1.2.3"},
		{"v1.2.3-beta.1", prefix + "1.2.3-beta.1"},
		{"", ""},
	}
	for _, c := range cases {
		if got := releaseNotesAPIURL(c.in); got != c.want {
			t.Fatalf("releaseNotesAPIURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// 非法字符被过滤，不会拼出路径穿越
	got := releaseNotesAPIURL("../../evil")
	if strings.Contains(got, "/../") {
		t.Fatalf("release notes url must not contain path traversal: %q", got)
	}
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("unexpected release notes url: %q", got)
	}
}

func TestHasMeaningfulNotes(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"**Full Changelog**: https://github.com/o/r/compare/v1...v2", false},
		{"## What's Changed\n\n**Full Changelog**: https://x/compare/v1...v2", false},
		{"", false},
		{"## What's Changed\n* Fix crash by @a\n", true},
		{"* 修复某个问题", true},
	}
	for _, c := range cases {
		if got := hasMeaningfulNotes(c.body); got != c.want {
			t.Fatalf("hasMeaningfulNotes(%q) = %v, want %v", c.body, got, c.want)
		}
	}
}

func TestReleaseNotesCompareURL(t *testing.T) {
	body := "**Full Changelog**: https://github.com/Heartfilia/LiteRedis/compare/v0.6.1...v0.6.2"
	want := "https://api.github.com/repos/Heartfilia/LiteRedis/compare/v0.6.1...v0.6.2"
	if got := releaseNotesCompareURL(body); got != want {
		t.Fatalf("releaseNotesCompareURL = %q, want %q", got, want)
	}
	if got := releaseNotesCompareURL("no link here"); got != "" {
		t.Fatalf("expected empty compare url, got %q", got)
	}
}

func TestBuildCommitNotes(t *testing.T) {
	var commits []compareCommit
	add := func(sha, msg string) {
		var c compareCommit
		c.SHA = sha
		c.Commit.Message = msg
		commits = append(commits, c)
	}
	add("1234567890abcdef", "Add wrap toggle to JSON tree\n\nlong body")
	add("abcdef1234567890", "Merge pull request #1 from x")
	add("deadbeefdeadbeef", "Fix update flow")

	out := buildCommitNotes(commits)
	if !strings.Contains(out, "* Add wrap toggle to JSON tree (1234567)") {
		t.Fatalf("missing first commit line: %q", out)
	}
	if strings.Contains(out, "Merge pull request") {
		t.Fatalf("merge commits should be skipped: %q", out)
	}
	if !strings.Contains(out, "* Fix update flow (deadbee)") {
		t.Fatalf("missing last commit line: %q", out)
	}
	if !strings.HasPrefix(out, "## 本次更新") {
		t.Fatalf("missing heading: %q", out)
	}
	if got := buildCommitNotes(nil); got != "" {
		t.Fatalf("empty commits should render empty, got %q", got)
	}
}
