package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTailFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "debug.log")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	lines, err := tailFile(path, 3)
	if err != nil {
		t.Fatalf("tailFile: %v", err)
	}
	want := []string{"line3", "line4", "line5"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d (%v)", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}

	empty := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	lines, err = tailFile(empty, 10)
	if err != nil || len(lines) != 0 {
		t.Fatalf("empty file should return no lines, got %v err=%v", lines, err)
	}

	if _, err := tailFile(filepath.Join(dir, "missing.log"), 10); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestTailFileLargeFileKeepsLastLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.log")
	var b strings.Builder
	for i := 0; i < 20000; i++ {
		b.WriteString(strings.Repeat("x", 40))
		b.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}

	lines, err := tailFile(path, 50)
	if err != nil {
		t.Fatalf("tailFile: %v", err)
	}
	if len(lines) != 50 {
		t.Fatalf("expected 50 lines, got %d", len(lines))
	}
	for _, line := range lines {
		if line != strings.Repeat("x", 40) {
			t.Fatalf("unexpected partial line: %q", line)
		}
	}
}
