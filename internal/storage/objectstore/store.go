package objectstore

import (
	"context"
	"strings"
)

type ObjectStore interface {
	Save(ctx context.Context, key string, data []byte) (string, error)
	Read(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	// Exists reports whether a file is stored under key (false, nil when it is not).
	Exists(ctx context.Context, key string) (bool, error)
	KeyFromURL(url string) (string, bool)
}

// contentTypeForKey picks the stored content type from the key's extension
// (PDF reports and the PNG/JPEG design images).
func contentTypeForKey(key string) string {
	switch {
	case strings.HasSuffix(key, ".png"):
		return "image/png"
	case strings.HasSuffix(key, ".jpg"), strings.HasSuffix(key, ".jpeg"):
		return "image/jpeg"
	}
	return "application/pdf"
}

func buildPublicURL(baseURL string, key string) string {
	base := trimRightSlash(baseURL)
	path := trimLeftSlash(key)
	return base + "/" + path
}

func trimRightSlash(value string) string {
	for len(value) > 1 && value[len(value)-1] == '/' {
		value = value[:len(value)-1]
	}
	return value
}

func trimLeftSlash(value string) string {
	for len(value) > 0 && value[0] == '/' {
		value = value[1:]
	}
	return value
}
