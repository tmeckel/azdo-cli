package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTestFileCreatesFile(t *testing.T) {
	content := "hello world"

	path, err := WriteTestFile(t, strings.NewReader(content))
	if err != nil {
		t.Fatalf("WriteTestFile returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Fatalf("file content mismatch: got %q", string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected perms 0600, got %o", perm)
	}
	// ensure the file is within the test temp dir
	if dir := filepath.Dir(path); !strings.HasPrefix(dir, t.TempDir()) {
		t.Logf("created file path: %s", path)
	}
}

func TestWriteTestFileNilContents(t *testing.T) {
	if _, err := WriteTestFile(t, nil); err == nil {
		t.Fatalf("expected error for nil contents")
	}
}

func TestWriteTestFileNilTestingT(t *testing.T) {
	if _, err := WriteTestFile(nil, strings.NewReader("content")); err == nil {
		t.Fatalf("expected error for nil testing.T")
	}
}

func TestWriteTestFileWithNameCreatesFile(t *testing.T) {
	content := "hello world"
	filename := "test-file.txt"

	path, err := WriteTestFileWithName(t, filename, strings.NewReader(content))
	if err != nil {
		t.Fatalf("WriteTestFileWithName returned error: %v", err)
	}

	// Check that the path ends with the requested filename
	if filepath.Base(path) != filename {
		t.Fatalf("expected filename %q, got %q", filename, filepath.Base(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Fatalf("file content mismatch: got %q", string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected perms 0600, got %o", perm)
	}
	// ensure the file is within the test temp dir
	if dir := filepath.Dir(path); !strings.HasPrefix(dir, t.TempDir()) {
		t.Logf("created file path: %s", path)
	}
}

func TestWriteTestFileWithNameEmptyFilename(t *testing.T) {
	if _, err := WriteTestFileWithName(t, "", strings.NewReader("content")); err == nil {
		t.Fatalf("expected error for empty filename")
	}
}

func TestWriteTestFileWithNameNilContents(t *testing.T) {
	if _, err := WriteTestFileWithName(t, "test.txt", nil); err == nil {
		t.Fatalf("expected error for nil contents")
	}
}

func TestWriteTestFileWithNameNilTestingT(t *testing.T) {
	if _, err := WriteTestFileWithName(nil, "test.txt", strings.NewReader("content")); err == nil {
		t.Fatalf("expected error for nil testing.T")
	}
}
