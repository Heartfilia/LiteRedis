//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// installDownloadedUpdate 挂载 dmg、把 .app 复制到目标位置旁边，
// 然后交给后台脚本：等当前进程退出后原子替换并重新打开应用。
func installDownloadedUpdate(assetPath string, asset ReleaseAsset) (string, bool, error) {
	if !strings.HasSuffix(strings.ToLower(asset.Name), ".dmg") {
		return "", false, fmt.Errorf("macOS 仅支持 dmg 更新包: %s", asset.Name)
	}
	bundle, err := currentAppBundle()
	if err != nil {
		return "", false, err
	}
	if strings.HasPrefix(bundle, "/Volumes/") {
		return "", false, fmt.Errorf("应用正在磁盘映像中运行，请先拖到「应用程序」后再更新")
	}
	if strings.Contains(bundle, "/AppTranslocation/") {
		return "", false, fmt.Errorf("应用被 Gatekeeper 限制在只读位置，请先执行 xattr -dr com.apple.quarantine \"/Applications/LiteRedis.app\" 后重新打开")
	}
	if err := ensureWritableDir(filepath.Dir(bundle)); err != nil {
		return "", false, err
	}

	mountPoint, err := os.MkdirTemp("", "literedis-mount-*")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(mountPoint)

	if out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", mountPoint, assetPath).CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("挂载更新包失败: %s", strings.TrimSpace(string(out)))
	}
	detached := false
	detach := func() {
		if detached {
			return
		}
		detached = true
		_ = exec.Command("hdiutil", "detach", mountPoint, "-force").Run()
	}
	defer detach()

	srcBundle, err := findAppBundle(mountPoint)
	if err != nil {
		return "", false, err
	}

	stagedBundle := bundle + ".update"
	_ = os.RemoveAll(stagedBundle)
	if out, err := exec.Command("ditto", srcBundle, stagedBundle).CombinedOutput(); err != nil {
		_ = os.RemoveAll(stagedBundle)
		return "", false, fmt.Errorf("复制新版本失败: %s", strings.TrimSpace(string(out)))
	}
	detach()

	script := buildApplyScript(os.Getpid(), bundle, stagedBundle, true)
	if err := runDetachedShell(script); err != nil {
		_ = os.RemoveAll(stagedBundle)
		return "", false, err
	}
	return "更新已就绪，应用正在重启", true, nil
}

// buildApplyScript 生成等待父进程退出后替换 .app 并重新拉起的脚本
func buildApplyScript(pid int, target, staged string, relaunch bool) string {
	launch := ""
	if relaunch {
		launch = "open \"$target\"\n"
	}
	return fmt.Sprintf(`#!/bin/sh
pid=%d
target=%s
staged=%s
if [ "$pid" -gt 0 ]; then
  while kill -0 "$pid" 2>/dev/null; do sleep 0.2; done
fi
rm -rf "$target.old"
if mv "$target" "$target.old" 2>/dev/null; then
  if ! mv "$staged" "$target"; then
    mv "$target.old" "$target"
    rm -rf "$staged"
    exit 1
  fi
  rm -rf "$target.old"
else
  rm -rf "$target"
  mv "$staged" "$target" || exit 1
fi
xattr -dr com.apple.quarantine "$target" 2>/dev/null
`+launch, pid, shellQuote(target), shellQuote(staged))
}

// currentAppBundle 从当前可执行文件反推出 .app 路径
func currentAppBundle() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}
	return appBundleForExecutable(execPath)
}

func appBundleForExecutable(execPath string) (string, error) {
	idx := strings.LastIndex(execPath, ".app/Contents/MacOS/")
	if idx < 0 {
		return "", fmt.Errorf("当前不是以 .app 形式运行，无法自动替换")
	}
	return execPath[:idx+len(".app")], nil
}

func findAppBundle(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			return filepath.Join(root, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("更新包中没有找到 .app")
}

func ensureWritableDir(dir string) error {
	file, err := os.CreateTemp(dir, ".literedis-write-test-*")
	if err != nil {
		return fmt.Errorf("安装目录不可写（%s），请把应用移动到「应用程序」后重试", dir)
	}
	name := file.Name()
	file.Close()
	return os.Remove(name)
}
