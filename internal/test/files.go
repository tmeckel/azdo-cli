package test

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// WriteTestFile writes the provided contents from the reader into a newly created
// random-named file inside t.TempDir(). The created file has restrictive permissions (0600).
// It returns the full path to the created file or an error.
func WriteTestFile(t *testing.T, contents io.Reader) (string, error) {
	if t == nil {
		return "", fmt.Errorf("t cannot be nil")
	}
	if contents == nil {
		return "", fmt.Errorf("contents cannot be nil")
	}

	dir := t.TempDir()

	// generate a short random filename
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate filename: %w", err)
	}
	name := hex.EncodeToString(b)
	path := filepath.Join(dir, name)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, contents); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return path, nil
}

// WriteTestFileWithName writes the provided contents from the reader into a file with the specified
// name inside t.TempDir(). The created file has restrictive permissions (0600).
// It returns the full path to the created file or an error.
func WriteTestFileWithName(t *testing.T, filename string, contents io.Reader) (string, error) {
	if t == nil {
		return "", fmt.Errorf("t cannot be nil")
	}
	if filename == "" {
		return "", fmt.Errorf("filename cannot be empty")
	}
	if contents == nil {
		return "", fmt.Errorf("contents cannot be nil")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, filename)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, contents); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return path, nil
}
