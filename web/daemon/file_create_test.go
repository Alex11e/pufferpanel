package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadOpenFlagsProtectExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, uploadOpenFlags(true), 0644)
	if f != nil {
		f.Close()
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("exclusive open error = %v, want os.ErrExist", err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "original" {
		t.Fatalf("original content changed: %q, %v", content, err)
	}
	newFile, err := os.OpenFile(filepath.Join(filepath.Dir(path), "new.txt"), uploadOpenFlags(true), 0644)
	if err != nil {
		t.Fatal(err)
	}
	if err := newFile.Close(); err != nil {
		t.Fatal(err)
	}
	existing, err := os.OpenFile(path, uploadOpenFlags(false), 0644)
	if err != nil {
		t.Fatal(err)
	}
	if err := existing.Close(); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(path)
	if err != nil || len(content) != 0 {
		t.Fatalf("normal upload did not truncate: %q, %v", content, err)
	}
}
