package token

import (
	"errors"
	"testing"
	"time"
)

func TestManagerGenerateAndParse(t *testing.T) {
	manager := NewManager("secret", time.Hour)

	value, err := manager.Generate("user-1", "test@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	claims, err := manager.Parse(value)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if claims.UserID != "user-1" {
		t.Fatalf("expected user id %q, got %q", "user-1", claims.UserID)
	}

	if claims.Email != "test@example.com" {
		t.Fatalf("expected email %q, got %q", "test@example.com", claims.Email)
	}
}

func TestManagerRejectsInvalidToken(t *testing.T) {
	manager := NewManager("secret", time.Hour)

	_, err := manager.Parse("not-a-token")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestManagerRejectsExpiredToken(t *testing.T) {
	manager := NewManager("secret", -time.Hour)

	value, err := manager.Generate("user-1", "test@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = manager.Parse(value)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestManagerRejectsWrongSecret(t *testing.T) {
	manager1 := NewManager("secret-1", time.Hour)
	manager2 := NewManager("secret-2", time.Hour)

	value, err := manager1.Generate("user-1", "test@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = manager2.Parse(value)
	if err == nil {
		t.Fatal("expected error")
	}

	if errors.Is(err, ErrInvalidToken) {
		return
	}
}
