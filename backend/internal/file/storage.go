package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStorage implements FileStorage using the local filesystem.
type LocalStorage struct {
	baseDir string
}

// NewLocalStorage creates a new LocalStorage rooted at the given base directory.
func NewLocalStorage(baseDir string) *LocalStorage {
	return &LocalStorage{baseDir: baseDir}
}

// Save writes the content from reader to disk at the given storage key path.
// It creates any necessary parent directories.
func (s *LocalStorage) Save(ctx context.Context, storageKey string, reader io.Reader) error {
	fullPath := filepath.Join(s.baseDir, storageKey)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("file storage: failed to create directory %s: %w", dir, err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("file storage: failed to create file %s: %w", fullPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("file storage: failed to write file %s: %w", fullPath, err)
	}

	return nil
}

// Open opens the file at the given storage key path for reading.
func (s *LocalStorage) Open(ctx context.Context, storageKey string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.baseDir, storageKey)

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("file storage: failed to open file %s: %w", fullPath, err)
	}

	return f, nil
}

// Delete removes the file at the given storage key path from disk.
func (s *LocalStorage) Delete(ctx context.Context, storageKey string) error {
	fullPath := filepath.Join(s.baseDir, storageKey)

	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("file storage: failed to delete file %s: %w", fullPath, err)
	}

	return nil
}

// Head checks if a file exists in the local filesystem.
func (s *LocalStorage) Head(ctx context.Context, storageKey string) error {
	fullPath := filepath.Join(s.baseDir, storageKey)
	_, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("local storage: file %s does not exist: %w", storageKey, err)
	}
	return nil
}
