package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicWriteStreamPreservesTargetOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("cancelled")
	err := AtomicWriteStream(path, 0600, func(w io.Writer) error {
		if _, err := io.WriteString(w, "partial data"); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "original" {
		t.Fatalf("target overwritten: %q %v", data, err)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary file leaked: %v %v", files, err)
	}
}

func TestAtomicWriteStreamReplacesWithPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteStream(path, 0600, func(w io.Writer) error {
		_, err := io.WriteString(w, "complete")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "complete" {
		t.Fatalf("data = %q %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("permissions = %o", info.Mode().Perm())
	}
}
