package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	updateDownloadTimeout = 15 * time.Minute
	updateCleanupDelay    = 30 * time.Minute
	updateTempMaxAge      = 24 * time.Hour
	updateTempPrefix      = "literedis-update-"
)

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
	Success    bool   `json:"success"`
	Message    string `json:"message,omitempty"`
	ReleaseURL string `json:"release_url,omitempty"`
	AssetName  string `json:"asset_name,omitempty"`
	AssetPath  string `json:"asset_path,omitempty"`
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
		return info
	}
	info.Latest = normalizeVersion(release.TagName)
	info.ReleaseURL = release.HTMLURL
	info.CheckedAt = time.Now().Format(time.RFC3339)
	info.NeedUpdate = compareSemver(normalizeVersion(info.Version), info.Latest) < 0
	return info
}

func (a *App) StartUpdate() UpdateResult {
	release, err := fetchLatestRelease(a.ctx, AppVersion)
	if err != nil {
		return UpdateResult{Success: false, Message: err.Error()}
	}

	asset := pickAsset(release.Assets)
	if asset == nil {
		return UpdateResult{Success: false, Message: "未找到适合当前平台的更新包", ReleaseURL: release.HTMLURL}
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
	if err := downloadFile(a.ctx, asset.BrowserDownloadURL, targetPath); err != nil {
		return UpdateResult{Success: false, Message: err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name}
	}
	checksumPath := filepath.Join(saveDir, checksumAsset.Name)
	if err := downloadFile(a.ctx, checksumAsset.BrowserDownloadURL, checksumPath); err != nil {
		return UpdateResult{Success: false, Message: "校验文件下载失败: " + err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name}
	}
	manifest, err := os.ReadFile(checksumPath)
	if err != nil {
		return UpdateResult{Success: false, Message: err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name}
	}
	if err := validateDownloadedAsset(targetPath, *asset, manifest); err != nil {
		return UpdateResult{Success: false, Message: "更新包校验失败: " + err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name}
	}

	if err := openInstaller(targetPath); err != nil {
		return UpdateResult{Success: false, Message: err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name, AssetPath: targetPath}
	}
	cleanup = false
	scheduleUpdateCleanup(saveDir, updateCleanupDelay)

	return UpdateResult{
		Success:    true,
		Message:    "更新包已下载并打开，请完成安装后重启应用",
		ReleaseURL: release.HTMLURL,
		AssetName:  asset.Name,
		AssetPath:  targetPath,
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

func fetchGitHubReleaseJSON(ctx context.Context, apiURL string, target any) error {
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LiteRedis-updater")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api status %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
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
	var generic *ReleaseAsset
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if !assetMatchesOS(name, goos) {
			continue
		}
		if strings.Contains(name, "universal") {
			return &assets[i]
		}
		if assetMatchesArch(name, goarch) {
			return &assets[i]
		}
		if !containsArchitectureToken(name) && generic == nil {
			generic = &assets[i]
		}
	}
	return generic
}

func assetMatchesOS(name, goos string) bool {
	switch goos {
	case "windows":
		return strings.Contains(name, "installer") && strings.HasSuffix(name, ".exe")
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
	client := &http.Client{Timeout: timeout}
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
