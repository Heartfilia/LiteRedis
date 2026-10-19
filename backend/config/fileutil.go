package config

import (
	"os"
	"path/filepath"
)

// AtomicWriteFile 使用临时文件 + rename 写盘，兼容 Windows 对覆盖写的限制。
//
// 兜底路径（Windows 上目标被 AV/同步盘占用导致 rename 失败）采用
// "旧文件先挪到 .bak → 再挪入新文件 → 失败则回滚 .bak" 的顺序，
// 任何一步失败都不会出现"旧配置已删、新配置未落盘"的丢失窗口。
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}

	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	}

	// 兜底：先把旧文件挪开（保留可回滚），再挪入新文件
	bakPath := path + ".replacing.bak"
	hadOld := false
	if err := os.Rename(path, bakPath); err == nil {
		hadOld = true
	} else if !os.IsNotExist(err) {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		// 回滚：旧文件原样放回，数据零丢失
		if hadOld {
			_ = os.Rename(bakPath, path)
		}
		cleanup()
		return err
	}
	if hadOld {
		_ = os.Remove(bakPath)
	}
	return nil
}
