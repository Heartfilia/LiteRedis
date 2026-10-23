package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"LiteRedis/backend/config"
	redisbackend "LiteRedis/backend/redis"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/mod/semver"
)

const (
	updateDownloadTimeout = 15 * time.Minute
	updateCleanupDelay    = 30 * time.Minute
	updateTempMaxAge      = 24 * time.Hour
	updateTempPrefix      = "literedis-update-"
	updateProgressEvent   = "update:progress"
	// updateReleaseCacheTTL 缓存 GitHub Release 响应，避免重复点击把 60 次/小时的配额打满
	updateReleaseCacheTTL = 2 * time.Minute
	updateReleaseMaxBytes = 4 << 20
)

type cachedReleaseResponse struct {
	body     []byte
	etag     string
	storedAt time.Time
}

var releaseResponseCache = struct {
	sync.Mutex
	entries map[string]cachedReleaseResponse
}{entries: make(map[string]cachedReleaseResponse)}

func loadReleaseCache(apiURL string) (cachedReleaseResponse, bool) {
	releaseResponseCache.Lock()
	defer releaseResponseCache.Unlock()
	entry, ok := releaseResponseCache.entries[apiURL]
	return entry, ok
}

func storeReleaseCache(apiURL string, body []byte, etag string) {
	releaseResponseCache.Lock()
	defer releaseResponseCache.Unlock()
	releaseResponseCache.entries[apiURL] = cachedReleaseResponse{body: body, etag: etag, storedAt: time.Now()}
}

func resetReleaseCacheForTest() {
	releaseResponseCache.Lock()
	defer releaseResponseCache.Unlock()
	releaseResponseCache.entries = make(map[string]cachedReleaseResponse)
}

// expireReleaseCacheForTest 把某条缓存标记为过期，用于验证 ETag 重新校验路径
func expireReleaseCacheForTest(apiURL string) {
	releaseResponseCache.Lock()
	defer releaseResponseCache.Unlock()
	entry, ok := releaseResponseCache.entries[apiURL]
	if !ok {
		return
	}
	entry.storedAt = time.Now().Add(-updateReleaseCacheTTL - time.Second)
	releaseResponseCache.entries[apiURL] = entry
}

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
	Size               int64  `json:"size"`
}

type ReleaseInfo struct {
	TagName    string         `json:"tag_name"`
	HTMLURL    string         `json:"html_url"`
	Assets     []ReleaseAsset `json:"assets"`
	Prerelease bool           `json:"prerelease"`
	Draft      bool           `json:"draft"`
}

type UpdateResult struct {
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	ReleaseURL   string `json:"release_url,omitempty"`
	AssetName    string `json:"asset_name,omitempty"`
	AssetPath    string `json:"asset_path,omitempty"`
	NeedsRestart bool   `json:"needs_restart,omitempty"`
}

// UpdateProgress 通过 update:progress 事件推送给前端
type UpdateProgress struct {
	Stage    string  `json:"stage"`
	Percent  float64 `json:"percent"`
	Received int64   `json:"received"`
	Total    int64   `json:"total"`
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	return strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
}

func canonicalSemver(version string) string {
	version = "v" + normalizeVersion(version)
	if !semver.IsValid(version) {
		return ""
	}
	return version
}

func compareSemver(a, b string) int {
	canonicalA := canonicalSemver(a)
	canonicalB := canonicalSemver(b)
	switch {
	case canonicalA != "" && canonicalB != "":
		return semver.Compare(canonicalA, canonicalB)
	case canonicalA == "" && canonicalB != "":
		return -1
	case canonicalA != "" && canonicalB == "":
		return 1
	case normalizeVersion(a) < normalizeVersion(b):
		return -1
	case normalizeVersion(a) > normalizeVersion(b):
		return 1
	default:
		return 0
	}
}

func isPrereleaseVersion(version string) bool {
	canonical := canonicalSemver(version)
	return canonical != "" && semver.Prerelease(canonical) != ""
}

func (a *App) CheckLatestRelease() VersionInfo {
	info := VersionInfo{Version: AppVersion}
	release, err := fetchLatestRelease(a.ctx, AppVersion)
	if err != nil {
		info.Error = err.Error()
		// 说明本次是否经过代理，方便排查"到底有没有走代理"
		if proxy := updateProxyHint(); proxy != "" {
			info.Error += "（已通过代理 " + proxy + " 访问）"
		}
		return info
	}
	info.Latest = normalizeVersion(release.TagName)
	info.ReleaseURL = release.HTMLURL
	info.CheckedAt = time.Now().Format(time.RFC3339)
	info.NeedUpdate = compareSemver(normalizeVersion(info.Version), info.Latest) < 0
	return info
}

// updateMu 防止双击"更新"并发触发两次下载与安装
var updateMu sync.Mutex

func (a *App) StartUpdate() UpdateResult {
	if !updateMu.TryLock() {
		return UpdateResult{Success: false, Message: "更新正在进行中，请稍候"}
	}
	defer updateMu.Unlock()

	emit := func(progress UpdateProgress) {
		if a.ctx == nil {
			return
		}
		wailsruntime.EventsEmit(a.ctx, updateProgressEvent, progress)
	}

	release, err := fetchLatestRelease(a.ctx, AppVersion)
	if err != nil {
		return UpdateResult{Success: false, Message: err.Error()}
	}

	asset := pickAsset(release.Assets)
	if asset == nil {
		message := "未找到适合当前平台的更新包"
		if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
			message = "当前版本暂未提供 macOS Intel 安装包，请到发布页手动下载或使用 Apple Silicon 设备"
		}
		return UpdateResult{Success: false, Message: message, ReleaseURL: release.HTMLURL}
	}
	checksumAsset := pickChecksumAsset(release.Assets)
	if checksumAsset == nil {
		return UpdateResult{Success: false, Message: "发布版本缺少 SHA256SUMS，已拒绝自动安装", ReleaseURL: release.HTMLURL}
	}

	_ = cleanupStaleUpdateDirs(os.TempDir(), time.Now().Add(-updateTempMaxAge))
	saveDir, err := os.MkdirTemp("", updateTempPrefix+"*")
	if err != nil {
		return UpdateResult{Success: false, Message: err.Error(), ReleaseURL: release.HTMLURL}
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(saveDir)
		}
	}()

	targetPath := filepath.Join(saveDir, asset.Name)
	emit(UpdateProgress{Stage: "download", Percent: 0, Total: asset.Size})
	if err := downloadFileWithProgress(a.ctx, asset.BrowserDownloadURL, targetPath, asset.Size, emit); err != nil {
		return UpdateResult{Success: false, Message: err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name}
	}
	checksumPath := filepath.Join(saveDir, checksumAsset.Name)
	if err := downloadFile(a.ctx, checksumAsset.BrowserDownloadURL, checksumPath); err != nil {
		return UpdateResult{Success: false, Message: "校验文件下载失败: " + err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name}
	}
	emit(UpdateProgress{Stage: "verify", Percent: 100})
	manifest, err := os.ReadFile(checksumPath)
	if err != nil {
		return UpdateResult{Success: false, Message: err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name}
	}
	if err := validateDownloadedAsset(targetPath, *asset, manifest); err != nil {
		return UpdateResult{Success: false, Message: "更新包校验失败: " + err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name}
	}

	emit(UpdateProgress{Stage: "install", Percent: 100})
	message, needsRestart, installErr := installDownloadedUpdate(targetPath, *asset)
	if installErr != nil {
		// 自动安装失败时退回旧行为：打开安装包让用户手动完成
		if err := openInstaller(targetPath); err != nil {
			return UpdateResult{Success: false, Message: "自动安装失败: " + installErr.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name, AssetPath: targetPath}
		}
		cleanup = false
		scheduleUpdateCleanup(saveDir, updateCleanupDelay)
		return UpdateResult{
			Success:    true,
			Message:    "自动安装失败（" + installErr.Error() + "），已为你打开安装包，请手动完成安装",
			ReleaseURL: release.HTMLURL,
			AssetName:  asset.Name,
			AssetPath:  targetPath,
		}
	}

	if needsRestart {
		// 给前端一点时间展示"正在重启"，随后由更新程序接管启动
		go func() {
			time.Sleep(600 * time.Millisecond)
			if a.ctx != nil {
				wailsruntime.Quit(a.ctx)
			}
		}()
	}

	return UpdateResult{
		Success:      true,
		Message:      message,
		ReleaseURL:   release.HTMLURL,
		AssetName:    asset.Name,
		AssetPath:    targetPath,
		NeedsRestart: needsRestart,
	}
}

func fetchLatestRelease(ctx context.Context, currentVersion string) (ReleaseInfo, error) {
	const (
		latestReleaseAPI = "https://api.github.com/repos/Heartfilia/LiteRedis/releases/latest"
		allReleasesAPI   = "https://api.github.com/repos/Heartfilia/LiteRedis/releases?per_page=100"
	)
	if !isPrereleaseVersion(currentVersion) {
		var release ReleaseInfo
		if err := fetchGitHubReleaseJSON(ctx, latestReleaseAPI, &release); err != nil {
			return ReleaseInfo{}, err
		}
		return release, nil
	}

	var releases []ReleaseInfo
	if err := fetchGitHubReleaseJSON(ctx, allReleasesAPI, &releases); err != nil {
		return ReleaseInfo{}, err
	}
	release, ok := selectLatestRelease(releases, true)
	if !ok {
		return ReleaseInfo{}, fmt.Errorf("github returned no valid releases")
	}
	return release, nil
}

// newUpdateHTTPClient 按应用设置决定是否走代理，仅作用于 GitHub 检查与安装包下载
func newUpdateHTTPClient(timeout time.Duration) (*http.Client, error) {
	settings, err := config.GetSettings()
	if err != nil {
		settings = config.AppSettings{}
	}
	return buildUpdateHTTPClient(settings, timeout)
}

// buildUpdateHTTPClient 便于单测覆盖的纯函数版本
func buildUpdateHTTPClient(settings config.AppSettings, timeout time.Duration) (*http.Client, error) {
	client := &http.Client{Timeout: timeout}
	if !settings.ProxyEnabled {
		return client, nil
	}
	proxyURL := strings.TrimSpace(settings.ProxyURL)
	if proxyURL == "" {
		// 开关打开却没填地址，直接报错，避免静默直连让用户以为代理生效了
		return nil, fmt.Errorf("已启用应用包更新代理，但代理地址为空")
	}
	// 拨号超时单独限制，别把 15 分钟的下载超时当成建连超时
	dialTimeout := timeout
	if dialTimeout <= 0 || dialTimeout > 30*time.Second {
		dialTimeout = 30 * time.Second
	}
	dialer, err := redisbackend.NewProxyDialer(proxyURL, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("代理配置无效: %w", err)
	}
	if dialer == nil {
		return client, nil
	}
	config.AppendDebugLog("[update] using proxy %s", redisbackend.RedactProxyURL(proxyURL))
	client.Transport = &http.Transport{
		DialContext:       dialer,
		ForceAttemptHTTP2: true,
	}
	return client, nil
}

// describeGitHubError 把 GitHub 的失败响应翻译成能定位问题的提示。
// 403 不一定就是配额用尽：代理节点被限制、出口 IP 触发风控、代理 ACL 拦截
// api.github.com 都会返回 403，所以这里要结合 X-RateLimit-* 头和响应体判断。
func describeGitHubError(resp *http.Response) error {
	snippet := ""
	if resp.Body != nil {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
		snippet = strings.TrimSpace(string(body))
	}
	proxy := updateProxyHint()
	viaProxy := ""
	if proxy != "" {
		viaProxy = "，本次经代理 " + proxy + " 访问"
	}
	remaining := strings.TrimSpace(resp.Header.Get("X-RateLimit-Remaining"))

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if remaining == "0" || resp.StatusCode == http.StatusTooManyRequests {
			reset := ""
			if ts, err := strconv.ParseInt(strings.TrimSpace(resp.Header.Get("X-RateLimit-Reset")), 10, 64); err == nil && ts > 0 {
				reset = "，约 " + time.Unix(ts, 0).Format("15:04") + " 恢复"
			}
			hint := ""
			if proxy != "" {
				hint = "。这是代理出口 IP 的配额，换节点或临时关闭代理直连都可以重试"
			}
			return fmt.Errorf("GitHub API 配额已用尽（未认证请求每个 IP 每小时 60 次%s）%s%s", reset, viaProxy, hint)
		}
		// 没有配额耗尽标记的 403：多半是代理/出口 IP 被拦
		message := "GitHub 返回 403，通常是代理出口 IP 被限制或代理屏蔽了 api.github.com" + viaProxy
		if snippet != "" {
			message += "，响应：" + snippet
		}
		return errors.New(message)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("GitHub 拒绝认证（401）%s", viaProxy)
	}
	if snippet != "" {
		return fmt.Errorf("github api status %s%s：%s", resp.Status, viaProxy, snippet)
	}
	return fmt.Errorf("github api status %s%s", resp.Status, viaProxy)
}

// updateProxyHint 返回脱敏后的代理地址，用于在错误信息里说明本次是否走了代理
func updateProxyHint() string {
	settings, err := config.GetSettings()
	if err != nil || !settings.ProxyEnabled {
		return ""
	}
	proxyURL := strings.TrimSpace(settings.ProxyURL)
	if proxyURL == "" {
		return ""
	}
	return redisbackend.RedactProxyURL(proxyURL)
}

// releaseCacheKey 把代理配置并入缓存键，切换代理后不会命中旧结果
func releaseCacheKey(apiURL string) string {
	settings, err := config.GetSettings()
	if err != nil || !settings.ProxyEnabled {
		return apiURL + "|direct"
	}
	return apiURL + "|proxy:" + strings.TrimSpace(settings.ProxyURL)
}

func fetchGitHubReleaseJSON(ctx context.Context, apiURL string, target any) error {
	cacheKey := releaseCacheKey(apiURL)
	cached, hasCached := loadReleaseCache(cacheKey)
	// 短时间内重复检查直接用缓存，完全不消耗 GitHub 配额
	if hasCached && time.Since(cached.storedAt) < updateReleaseCacheTTL {
		return json.Unmarshal(cached.body, target)
	}

	client, err := newUpdateHTTPClient(12 * time.Second)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LiteRedis-updater")
	// 有缓存时带上 ETag，未变化会返回 304，省流量也省配额
	if hasCached && cached.etag != "" {
		req.Header.Set("If-None-Match", cached.etag)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && hasCached {
		storeReleaseCache(cacheKey, cached.body, cached.etag)
		return json.Unmarshal(cached.body, target)
	}
	if resp.StatusCode != http.StatusOK {
		return describeGitHubError(resp)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, updateReleaseMaxBytes))
	if err != nil {
		return err
	}
	storeReleaseCache(cacheKey, body, resp.Header.Get("ETag"))
	return json.Unmarshal(body, target)
}

func selectLatestRelease(releases []ReleaseInfo, includePrerelease bool) (ReleaseInfo, bool) {
	var selected ReleaseInfo
	selectedVersion := ""
	for _, release := range releases {
		version := canonicalSemver(release.TagName)
		if release.Draft || version == "" || (!includePrerelease && release.Prerelease) {
			continue
		}
		if selectedVersion == "" || semver.Compare(version, selectedVersion) > 0 {
			selected = release
			selectedVersion = version
		}
	}
	return selected, selectedVersion != ""
}

func pickAsset(assets []ReleaseAsset) *ReleaseAsset {
	return pickAssetForPlatform(assets, runtime.GOOS, runtime.GOARCH)
}

func pickAssetForPlatform(assets []ReleaseAsset, goos, goarch string) *ReleaseAsset {
	var generic, archMatch, universal, portable *ReleaseAsset
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if !assetMatchesOS(name, goos) {
			continue
		}
		// Windows 优先选裸 exe：可以原地替换实现静默更新，NSIS 安装包作为兜底
		if goos == "windows" && !strings.Contains(name, "installer") && portable == nil {
			portable = &assets[i]
		}
		if strings.Contains(name, "universal") && universal == nil {
			universal = &assets[i]
		}
		if assetMatchesArch(name, goarch) && archMatch == nil {
			archMatch = &assets[i]
		}
		if !containsArchitectureToken(name) && generic == nil {
			generic = &assets[i]
		}
	}
	switch {
	case goos == "windows" && portable != nil:
		return portable
	case universal != nil:
		return universal
	case archMatch != nil:
		return archMatch
	default:
		return generic
	}
}

func assetMatchesOS(name, goos string) bool {
	switch goos {
	case "windows":
		return strings.HasSuffix(name, ".exe")
	case "darwin":
		return strings.HasSuffix(name, ".dmg")
	case "linux":
		return strings.HasSuffix(name, ".tar.gz")
	}
	return false
}

func assetMatchesArch(name, goarch string) bool {
	switch goarch {
	case "arm64":
		return strings.Contains(name, "arm64") || strings.Contains(name, "aarch64")
	case "amd64":
		return strings.Contains(name, "amd64") || strings.Contains(name, "x86_64") || strings.Contains(name, "x64") || strings.Contains(name, "intel")
	case "386":
		return strings.Contains(name, "386") || strings.Contains(name, "x86")
	default:
		return strings.Contains(name, goarch)
	}
}

func containsArchitectureToken(name string) bool {
	for _, token := range []string{"arm64", "aarch64", "amd64", "x86_64", "x64", "intel", "386", "x86"} {
		if strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func pickChecksumAsset(assets []ReleaseAsset) *ReleaseAsset {
	for i := range assets {
		if strings.EqualFold(strings.TrimSpace(assets[i].Name), "SHA256SUMS") {
			return &assets[i]
		}
	}
	return nil
}

func validateDownloadedAsset(path string, asset ReleaseAsset, manifest []byte) error {
	expected, err := checksumForAsset(manifest, asset.Name)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if asset.Size > 0 && info.Size() != asset.Size {
		return fmt.Errorf("size mismatch for %s: got %d, want %d", asset.Name, info.Size(), asset.Size)
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := fmt.Sprintf("%x", hash.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(actual)), []byte(strings.ToLower(expected))) != 1 {
		return fmt.Errorf("SHA-256 mismatch for %s", asset.Name)
	}
	return nil
}

func checksumForAsset(manifest []byte, assetName string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(manifest)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if filepath.Base(name) != filepath.Base(assetName) {
			continue
		}
		digest := strings.ToLower(fields[0])
		if len(digest) != sha256.Size*2 {
			return "", fmt.Errorf("invalid SHA-256 entry for %s", assetName)
		}
		for _, char := range digest {
			if !strings.ContainsRune("0123456789abcdef", char) {
				return "", fmt.Errorf("invalid SHA-256 entry for %s", assetName)
			}
		}
		return digest, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("SHA-256 entry not found for %s", assetName)
}

func downloadFile(ctx context.Context, url, target string) error {
	return downloadFileWithTimeout(ctx, url, target, updateDownloadTimeout)
}

func downloadFileWithTimeout(ctx context.Context, url, target string, timeout time.Duration) error {
	client, err := newUpdateHTTPClient(timeout)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LiteRedis-updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// downloadFileWithProgress 下载更新包并按 ~5 次/秒节流上报进度
func downloadFileWithProgress(ctx context.Context, url, target string, total int64, emit func(UpdateProgress)) error {
	client, err := newUpdateHTTPClient(updateDownloadTimeout)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LiteRedis-updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	if total <= 0 {
		total = resp.ContentLength
	}

	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := &progressWriter{total: total, emit: emit}
	if _, err := io.Copy(io.MultiWriter(f, writer), resp.Body); err != nil {
		return err
	}
	if emit != nil {
		emit(UpdateProgress{Stage: "download", Percent: 100, Received: writer.received, Total: total})
	}
	return nil
}

type progressWriter struct {
	total    int64
	received int64
	lastEmit time.Time
	emit     func(UpdateProgress)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.received += int64(len(p))
	if w.emit != nil && time.Since(w.lastEmit) >= 200*time.Millisecond {
		w.lastEmit = time.Now()
		w.emit(UpdateProgress{Stage: "download", Percent: percentOf(w.received, w.total), Received: w.received, Total: w.total})
	}
	return len(p), nil
}

func percentOf(received, total int64) float64 {
	if total <= 0 {
		return 0
	}
	percent := float64(received) / float64(total) * 100
	if percent > 100 {
		return 100
	}
	return percent
}

// copyFile 以 0644 权限复制文件（用于跨分区放置待替换的可执行文件）
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}

// extractTarGzBinary 从 tar.gz 中取出指定名称的文件到 dst
func extractTarGzBinary(archivePath, memberName, dst string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != memberName {
			continue
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, reader); err != nil {
			out.Close()
			_ = os.Remove(dst)
			return err
		}
		return out.Close()
	}
	return fmt.Errorf("压缩包中未找到 %s", memberName)
}

func cleanupStaleUpdateDirs(root string, cutoff time.Time) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var firstErr error
	for _, entry := range entries {
		if !entry.IsDir() || (!strings.HasPrefix(entry.Name(), updateTempPrefix) && !strings.HasPrefix(entry.Name(), "litetredis-update-")) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func scheduleUpdateCleanup(path string, delay time.Duration) {
	time.AfterFunc(delay, func() {
		_ = os.RemoveAll(path)
	})
}

func openInstaller(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command(path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	case "linux":
		return exec.Command("xdg-open", path).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
