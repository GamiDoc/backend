package auth

import (
	"context"
	"log/slog"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// AccountFiles lists what a user's projects reference in the object storage: the file
// URLs saved by the server, and the design data (JSON) of each project, which contains
// the URLs of its uploaded images.
type AccountFiles interface {
	StoredFiles(ctx context.Context, userID string) (files []string, designs map[string]string, err error)
}

// FileStore deletes stored files (PDFs, design documents, images) by URL.
type FileStore interface {
	KeyFromURL(url string) (string, bool)
	Delete(ctx context.Context, key string) error
}

// WithFileCleanup makes DeleteAccount also remove the user's stored files.
func (s *Service) WithFileCleanup(files AccountFiles, store FileStore) *Service {
	s.files = files
	s.store = store
	return s
}

// DeleteAccount removes the account after checking the password: the user, their
// projects with designs, evaluations and documents, and the stored files. The current
// session is closed.
func (s *Service) DeleteAccount(ctx context.Context, userID, password, accessToken, refreshToken string) error {
	found, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(found.PasswordHash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}

	// Read the files first: they are no longer reachable once the rows are gone.
	var keys []string
	if s.files != nil && s.store != nil {
		files, designs, err := s.files.StoredFiles(ctx, userID)
		if err != nil {
			return err
		}
		keys = s.storedKeys(files, designs)
	}
	if err := s.users.Delete(ctx, userID); err != nil {
		return err
	}
	_ = s.Logout(ctx, accessToken, refreshToken)

	// Best effort: a file left behind is not reachable from any account any more.
	cleanup := context.WithoutCancel(ctx)
	for _, key := range keys {
		if err := s.store.Delete(cleanup, key); err != nil {
			slog.Warn("account deletion: stored file not deleted", "key", key, "error", err)
		}
	}
	return nil
}

// imageURL finds URLs and site-relative paths inside a design (JSON).
var imageURL = regexp.MustCompile(`(?:https?://|/)[^"\s\\<>]+`)

// storedKeys returns the object storage keys to delete, once each: the files the server
// saved for the projects, and the images inside each project's own image folder. The
// design data is written by the user, so a URL there pointing anywhere else (another
// project's image, a document) is left alone.
func (s *Service) storedKeys(files []string, designs map[string]string) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(key string) {
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for _, url := range files {
		if key, ok := s.store.KeyFromURL(url); ok {
			add(key)
		}
	}
	for projectID, data := range designs {
		folder := "design/projects/" + projectID + "/images/"
		for _, url := range imageURL.FindAllString(data, -1) {
			if key, ok := s.store.KeyFromURL(url); ok && strings.HasPrefix(key, folder) && !strings.Contains(key, "..") {
				add(key)
			}
		}
	}
	return keys
}
