package main

import (
	"context"
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
)

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
	Size               int64  `json:"size"`
}

type ReleaseInfo struct {
	TagName string         `json:"tag_name"`
	HTMLURL string         `json:"html_url"`
	Assets  []ReleaseAsset `json:"assets"`
}

type UpdateResult struct {
	Success    bool   `json:"success"`
	Message    string `json:"message,omitempty"`
	ReleaseURL string `json:"release_url,omitempty"`
	AssetName  string `json:"asset_name,omitempty"`
	AssetPath  string `json:"asset_path,omitempty"`
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func compareSemver(a, b string) int {
	parse := func(v string) [3]int {
		var out [3]int
		parts := strings.Split(normalizeVersion(v), ".")
		for i := 0; i < len(parts) && i < 3; i++ {
			fmt.Sscanf(parts[i], "%d", &out[i])
		}
		return out
	}
	va := parse(a)
	vb := parse(b)
	for i := 0; i < 3; i++ {
		if va[i] < vb[i] {
			return -1
		}
		if va[i] > vb[i] {
			return 1
		}
	}
	return 0
}

func (a *App) CheckLatestRelease() VersionInfo {
	info := VersionInfo{Version: AppVersion}
	release, err := fetchLatestRelease(a.ctx)
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
	release, err := fetchLatestRelease(a.ctx)
	if err != nil {
		return UpdateResult{Success: false, Message: err.Error()}
	}

	asset := pickAsset(release.Assets)
	if asset == nil {
		return UpdateResult{Success: false, Message: "未找到适合当前平台的更新包", ReleaseURL: release.HTMLURL}
	}

	saveDir, err := os.MkdirTemp("", "litetredis-update-*")
	if err != nil {
		return UpdateResult{Success: false, Message: err.Error(), ReleaseURL: release.HTMLURL}
	}

	targetPath := filepath.Join(saveDir, asset.Name)
	if err := downloadFile(a.ctx, asset.BrowserDownloadURL, targetPath); err != nil {
		return UpdateResult{Success: false, Message: err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name}
	}

	if err := openInstaller(targetPath); err != nil {
		return UpdateResult{Success: false, Message: err.Error(), ReleaseURL: release.HTMLURL, AssetName: asset.Name, AssetPath: targetPath}
	}

	return UpdateResult{
		Success:    true,
		Message:    "更新包已下载并打开，请完成安装后重启应用",
		ReleaseURL: release.HTMLURL,
		AssetName:  asset.Name,
		AssetPath:  targetPath,
	}
}

func fetchLatestRelease(ctx context.Context) (ReleaseInfo, error) {
	const repoAPI = "https://api.github.com/repos/Heartfilia/LiteRedis/releases/latest"

	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoAPI, nil)
	if err != nil {
		return ReleaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LiteRedis-updater")

	resp, err := client.Do(req)
	if err != nil {
		return ReleaseInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ReleaseInfo{}, fmt.Errorf("github api status %s", resp.Status)
	}

	var out ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ReleaseInfo{}, err
	}
	return out, nil
}

func pickAsset(assets []ReleaseAsset) *ReleaseAsset {
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		switch runtime.GOOS {
		case "windows":
			if strings.Contains(name, "installer") && strings.HasSuffix(name, ".exe") {
				return &assets[i]
			}
		case "darwin":
			if strings.HasSuffix(name, ".dmg") {
				return &assets[i]
			}
		case "linux":
			if strings.HasSuffix(name, ".tar.gz") {
				return &assets[i]
			}
		}
	}
	return nil
}

func downloadFile(ctx context.Context, url, target string) error {
	client := &http.Client{Timeout: 0}
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
