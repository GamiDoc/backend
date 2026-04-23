package objectstore

import "context"

type ObjectStore interface {
	Save(ctx context.Context, key string, data []byte) (string, error)
	Read(ctx context.Context, key string) ([]byte, error)
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
