package objectstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStoreSaveAndRead(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "pdfs")
	store := NewLocalStore(rootDir, "/files/pdfs")

	url, err := store.Save(context.Background(), "projects/test/file.pdf", []byte("hello"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if url != "/files/pdfs/projects/test/file.pdf" {
		t.Fatalf("unexpected url %q", url)
	}

	data, err := store.Read(context.Background(), "projects/test/file.pdf")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if string(data) != "hello" {
		t.Fatalf("unexpected file content %q", string(data))
	}

	if _, err := os.Stat(filepath.Join(rootDir, "projects", "test", "file.pdf")); err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}
}
