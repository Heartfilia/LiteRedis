//go:build darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildApplyScriptSwapsBundle(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "LiteRedis.app")
	staged := filepath.Join(dir, "LiteRedis.app.update")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "old.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staged, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "new.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(dir, "apply.sh")
	if err := os.WriteFile(scriptPath, []byte(buildApplyScript(0, target, staged, false)), 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/bin/sh", scriptPath).CombinedOutput(); err != nil {
		t.Fatalf("apply script failed: %v: %s", err, out)
	}

	if _, err := os.Stat(filepath.Join(target, "new.txt")); err != nil {
		t.Fatalf("new bundle was not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "old.txt")); !os.IsNotExist(err) {
		t.Fatal("old bundle content leaked into new bundle")
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Fatal("backup bundle was not cleaned up")
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatal("staged bundle was not moved")
	}
}

func TestBuildApplyScriptHandlesQuotedPaths(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "weird 'dir'")
	target := filepath.Join(dir, "App.app")
	staged := filepath.Join(dir, "App.app.update")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staged, 0755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "apply.sh")
	if err := os.WriteFile(scriptPath, []byte(buildApplyScript(0, target, staged, false)), 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/bin/sh", scriptPath).CombinedOutput(); err != nil {
		t.Fatalf("apply script failed: %v: %s", err, out)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target missing after swap: %v", err)
	}
}

func TestAppBundleForExecutable(t *testing.T) {
	got, err := appBundleForExecutable("/Applications/LiteRedis.app/Contents/MacOS/LiteRedis")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/Applications/LiteRedis.app" {
		t.Fatalf("bundle = %q", got)
	}
	if _, err := appBundleForExecutable("/Users/me/go/bin/LiteRedis"); err == nil {
		t.Fatal("non-bundle executable was accepted")
	}
}
