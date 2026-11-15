package auth

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gamidoc/backend/internal/mailer"
	"github.com/gamidoc/backend/internal/token"
	"github.com/gamidoc/backend/internal/user"
	goredis "github.com/redis/go-redis/v9"
)

type recordingMailer struct {
	mu   sync.Mutex
	sent []mailer.Message
}

func (m *recordingMailer) Send(ctx context.Context, message mailer.Message) (mailer.SendResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, message)
	return mailer.SendResult{Provider: "test", Accepted: true}, nil
}

var linkToken = regexp.MustCompile(`\?(verify|reset)=([0-9a-f]+)`)

// lastToken returns the token of the link in the last email sent.
func (m *recordingMailer) lastToken(t *testing.T, kind string) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no email sent")
	}
	match := linkToken.FindStringSubmatch(m.sent[len(m.sent)-1].Text)
	if match == nil || match[1] != kind {
		t.Fatalf("no %s link in %q", kind, m.sent[len(m.sent)-1].Text)
	}
	value, _ := url.QueryUnescape(match[2])
	return value
}

func newEmailService(t *testing.T) (*Service, *recordingMailer, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &fakeUserRepository{usersByEmail: map[string]user.User{}, usersByID: map[string]user.User{}}
	m := &recordingMailer{}
	service := NewService(repo, token.NewManager("secret", time.Hour), token.NewRefreshStore(rdb, time.Hour), token.NewBlacklist(rdb)).
		WithEmail(m, token.NewOneTimeStore(rdb), "noreply@example.com", "GamiDoc", "https://app.example.com/")
	return service, m, mr
}

func TestValidatePassword(t *testing.T) {
	for password, ok := range map[string]bool{
		"Password1":              true,
		"password1":              false, // no uppercase
		"PASSWORD1":              false, // no lowercase
		"Password":               false, // no number
		"Pass1":                  false, // too short
		"Àbcdefg1":               true,
		string(make([]byte, 80)): false,
	} {
		if err := ValidatePassword(password); (err == nil) != ok {
			t.Errorf("ValidatePassword(%q) = %v, want ok=%v", password, err, ok)
		}
	}
}

func TestRegisterSendsVerificationAndBlocksLogin(t *testing.T) {
	ctx := context.Background()
	service, m, _ := newEmailService(t)

	result, err := service.Register(ctx, RegisterInput{Email: "New@Example.com", Password: "Password123"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !result.VerificationRequired || result.AccessToken != "" {
		t.Fatalf("expected verification required and no token, got %+v", result)
	}
	if len(m.sent) != 1 || m.sent[0].To[0] != "new@example.com" {
		t.Fatalf("expected one email to new@example.com, got %+v", m.sent)
	}
	if !regexp.MustCompile(`^https://app\.example\.com/\?verify=`).MatchString(linkToken.FindString(m.sent[0].Text)) &&
		!regexp.MustCompile(`https://app\.example\.com/\?verify=`).MatchString(m.sent[0].Text) {
		t.Fatalf("link does not point to the frontend: %q", m.sent[0].Text)
	}

	if _, err := service.Login(ctx, LoginInput{Email: "new@example.com", Password: "Password123"}); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
	// A wrong password still reads as wrong credentials, not as "not verified".
	if _, err := service.Login(ctx, LoginInput{Email: "new@example.com", Password: "Wrong1234"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	verified, err := service.VerifyEmail(ctx, m.lastToken(t, "verify"))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verified.AccessToken == "" || !verified.User.Verified() {
		t.Fatalf("expected a logged-in verified user, got %+v", verified)
	}
	if _, err := service.Login(ctx, LoginInput{Email: "new@example.com", Password: "Password123"}); err != nil {
		t.Fatalf("login after verification: %v", err)
	}
	// The link works once.
	if _, err := service.VerifyEmail(ctx, m.lastToken(t, "verify")); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken on reuse, got %v", err)
	}
}

func TestResendVerificationCooldownAndOldLink(t *testing.T) {
	ctx := context.Background()
	service, m, mr := newEmailService(t)
	if _, err := service.Register(ctx, RegisterInput{Email: "a@example.com", Password: "Password123"}); err != nil {
		t.Fatal(err)
	}
	first := m.lastToken(t, "verify")

	if err := service.ResendVerification(ctx, "a@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := service.ResendVerification(ctx, "a@example.com"); err != nil {
		t.Fatal(err)
	}
	if len(m.sent) != 2 {
		t.Fatalf("expected 2 emails (second resend in cooldown), got %d", len(m.sent))
	}
	mr.FastForward(2 * time.Minute)
	if err := service.ResendVerification(ctx, "unknown@example.com"); err != nil {
		t.Fatalf("unknown address must not fail: %v", err)
	}
	if len(m.sent) != 2 {
		t.Fatalf("no email for an unknown address, got %d", len(m.sent))
	}
	// Only the newest link works.
	if _, err := service.VerifyEmail(ctx, first); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old link should be invalid, got %v", err)
	}
	if _, err := service.VerifyEmail(ctx, m.lastToken(t, "verify")); err != nil {
		t.Fatalf("new link: %v", err)
	}
}

func TestForgotAndResetPassword(t *testing.T) {
	ctx := context.Background()
	service, m, mr := newEmailService(t)
	if _, err := service.Register(ctx, RegisterInput{Email: "b@example.com", Password: "Password123"}); err != nil {
		t.Fatal(err)
	}

	if err := service.ForgotPassword(ctx, "nobody@example.com"); err != nil || len(m.sent) != 1 {
		t.Fatalf("unknown address: err=%v sent=%d", err, len(m.sent))
	}
	if err := service.ForgotPassword(ctx, " B@example.com "); err != nil {
		t.Fatal(err)
	}
	reset := m.lastToken(t, "reset")
	if err := service.CheckResetToken(ctx, reset); err != nil {
		t.Fatalf("check: %v", err)
	}
	if _, err := service.ResetPassword(ctx, reset, "weak"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
	result, err := service.ResetPassword(ctx, reset, "NewPassword9")
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if result.AccessToken == "" || !result.User.Verified() {
		t.Fatalf("expected a logged-in verified user, got %+v", result)
	}
	if _, err := service.Login(ctx, LoginInput{Email: "b@example.com", Password: "Password123"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password should fail, got %v", err)
	}
	if _, err := service.Login(ctx, LoginInput{Email: "b@example.com", Password: "NewPassword9"}); err != nil {
		t.Fatalf("new password: %v", err)
	}
	if _, err := service.ResetPassword(ctx, reset, "Another123"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reset link must work once, got %v", err)
	}

	// Links expire.
	mr.FastForward(2 * time.Minute)
	if err := service.ForgotPassword(ctx, "b@example.com"); err != nil {
		t.Fatal(err)
	}
	expiring := m.lastToken(t, "reset")
	mr.FastForward(61 * time.Minute)
	if err := service.CheckResetToken(ctx, expiring); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected an expired link, got %v", err)
	}
}
