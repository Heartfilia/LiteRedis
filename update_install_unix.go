//go:build darwin || linux

package main

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// shellQuote 把任意字符串安全地包成 POSIX shell 单引号字面量
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// runDetachedShell 在独立会话中执行脚本，保证主进程退出后更新脚本仍能继续
func runDetachedShell(script string) error {
	file, err := os.CreateTemp("", "literedis-apply-*.sh")
	if err != nil {
		return err
	}
	path := file.Name()
	if _, err := file.WriteString(script); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(path, 0700); err != nil {
		return err
	}
	cmd := exec.Command("/bin/sh", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
