package objectstore

import (
	"context"
	"os"
	"path/filepath"
)

type LocalStore struct {
	rootDir string
	baseURL string
}

func NewLocalStore(rootDir string, baseURL string) *LocalStore {
	return &LocalStore{
		rootDir: rootDir,
		baseURL: baseURL,
	}
}

func (s *LocalStore) Save(ctx context.Context, key string, data []byte) (string, error) {
	path := filepath.Join(s.rootDir, filepath.FromSlash(trimLeftSlash(key)))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}

	return buildPublicURL(s.baseURL, key), nil
}

func (s *LocalStore) Read(ctx context.Context, key string) ([]byte, error) {
	path := filepath.Join(s.rootDir, filepath.FromSlash(trimLeftSlash(key)))
	return os.ReadFile(path)
}
