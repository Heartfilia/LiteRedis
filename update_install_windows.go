//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const detachProcess = 0x00000008

func installDownloadedUpdate(assetPath string, asset ReleaseAsset) (string, bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	if strings.Contains(strings.ToLower(asset.Name), "installer") {
		return runSilentInstaller(assetPath)
	}

	stagePath := exePath + ".new"
	_ = os.Remove(stagePath)
	if err := copyFile(assetPath, stagePath); err != nil {
		return "", false, fmt.Errorf("准备新版本失败: %w", err)
	}

	oldPath := exePath + ".old"
	_ = os.Remove(oldPath)
	// Windows 允许重命名正在运行的 exe，因此可以同步完成替换
	if err := os.Rename(exePath, oldPath); err != nil {
		_ = os.Remove(stagePath)
		return "", false, fmt.Errorf("替换可执行文件失败: %w", err)
	}
	if err := os.Rename(stagePath, exePath); err != nil {
		_ = os.Rename(oldPath, exePath)
		_ = os.Remove(stagePath)
		return "", false, fmt.Errorf("写入新版本失败: %w", err)
	}

	script := fmt.Sprintf("@echo off\r\nping -n 3 127.0.0.1 >nul\r\nstart \"\" \"%s\"\r\ndel /f /q \"%s\" >nul 2>&1\r\n", exePath, oldPath)
	batPath := filepath.Join(os.TempDir(), "literedis-relaunch.bat")
	if err := os.WriteFile(batPath, []byte(script), 0600); err != nil {
		return "", false, err
	}
	cmd := exec.Command("cmd", "/c", batPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: detachProcess}
	if err := cmd.Start(); err != nil {
		if startErr := exec.Command(exePath).Start(); startErr != nil {
			return "", false, startErr
		}
	}
	return "更新完成，应用正在重启", true, nil
}

// runSilentInstaller 没有裸 exe 时退回 NSIS 静默安装
func runSilentInstaller(installerPath string) (string, bool, error) {
	script := fmt.Sprintf("@echo off\r\n:wait\r\ntasklist /FI \"PID eq %d\" 2>nul | find \"%d\" >nul\r\nif not errorlevel 1 (\r\n  ping -n 2 127.0.0.1 >nul\r\n  goto wait\r\n)\r\nstart \"\" \"%s\" /S\r\n",
		os.Getpid(), os.Getpid(), installerPath)
	batPath := filepath.Join(os.TempDir(), "literedis-install.bat")
	if err := os.WriteFile(batPath, []byte(script), 0600); err != nil {
		return "", false, err
	}
	cmd := exec.Command("cmd", "/c", batPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: detachProcess}
	if err := cmd.Start(); err != nil {
		return "", false, err
	}
	return "安装程序已启动，应用正在重启", true, nil
}
