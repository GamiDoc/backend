package auth

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/gamidoc/backend/internal/user"
)

type fakeAccountFiles struct {
	files   []string
	designs map[string]string
}

func (f fakeAccountFiles) StoredFiles(ctx context.Context, userID string) ([]string, map[string]string, error) {
	return f.files, f.designs, nil
}

type fakeFileStore struct{ deleted []string }

func (s *fakeFileStore) KeyFromURL(url string) (string, bool) {
	const base = "https://cdn.example.com/files/"
	if !strings.HasPrefix(url, base) || len(url) == len(base) {
		return "", false
	}
	return url[len(base):], true
}

func (s *fakeFileStore) Delete(ctx context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

func TestDeleteAccount(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	registered, err := service.Register(ctx, RegisterInput{Email: "gone@example.com", Password: "Password123"})
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeFileStore{}
	service.WithFileCleanup(fakeAccountFiles{
		files: []string{
			"https://cdn.example.com/files/evaluation/p1.pdf",
			"https://cdn.example.com/files/design/p1/report.pdf",
			"https://elsewhere.example.com/x.pdf",
		},
		designs: map[string]string{
			"p1": `{"cover":{"image":"https://cdn.example.com/files/design/projects/p1/images/a.png"},` +
				`"other":"https://cdn.example.com/files/design/projects/p2/images/not-mine.png",` +
				`"doc":"https://cdn.example.com/files/design/p9/report.pdf",` +
				`"again":"https://cdn.example.com/files/design/projects/p1/images/a.png"}`,
		},
	}, store)

	if err := service.DeleteAccount(ctx, registered.User.ID, "Wrong1234", registered.AccessToken, registered.RefreshToken); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if len(store.deleted) != 0 {
		t.Fatal("nothing must be deleted with a wrong password")
	}

	if err := service.DeleteAccount(ctx, registered.User.ID, "Password123", registered.AccessToken, registered.RefreshToken); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := service.Me(ctx, registered.User.ID); !errors.Is(err, user.ErrUserNotFound) {
		t.Fatalf("user should be gone, got %v", err)
	}
	if _, err := service.Login(ctx, LoginInput{Email: "gone@example.com", Password: "Password123"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("login after deletion: %v", err)
	}
	if _, err := service.Refresh(ctx, registered.RefreshToken); err == nil {
		t.Fatal("the refresh token should be revoked")
	}

	sort.Strings(store.deleted)
	want := []string{"design/p1/report.pdf", "design/projects/p1/images/a.png", "evaluation/p1.pdf"}
	if strings.Join(store.deleted, ",") != strings.Join(want, ",") {
		t.Fatalf("deleted %v, want %v (only own files)", store.deleted, want)
	}
}
