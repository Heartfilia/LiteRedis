//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func installDownloadedUpdate(assetPath string, asset ReleaseAsset) (string, bool, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}

	stagePath := execPath + ".new"
	_ = os.Remove(stagePath)
	if strings.HasSuffix(strings.ToLower(asset.Name), ".tar.gz") {
		if err := extractTarGzBinary(assetPath, filepath.Base(execPath), stagePath); err != nil {
			return "", false, err
		}
	} else if err := copyFile(assetPath, stagePath); err != nil {
		return "", false, err
	}
	if err := os.Chmod(stagePath, 0755); err != nil {
		_ = os.Remove(stagePath)
		return "", false, err
	}

	oldPath := execPath + ".old"
	_ = os.Remove(oldPath)
	if err := os.Rename(execPath, oldPath); err != nil {
		_ = os.Remove(stagePath)
		return "", false, fmt.Errorf("替换可执行文件失败: %w", err)
	}
	if err := os.Rename(stagePath, execPath); err != nil {
		_ = os.Rename(oldPath, execPath)
		_ = os.Remove(stagePath)
		return "", false, fmt.Errorf("写入新版本失败: %w", err)
	}

	script := fmt.Sprintf("pid=%d\nwhile kill -0 \"$pid\" 2>/dev/null; do sleep 0.2; done\nrm -f %s\nexec %s\n",
		os.Getpid(), shellQuote(oldPath), shellQuote(execPath))
	if err := runDetachedShell(script); err != nil {
		return "", false, err
	}
	return "更新完成，应用正在重启", true, nil
}
