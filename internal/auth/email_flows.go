package auth

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gamidoc/backend/internal/mailer"
	"github.com/gamidoc/backend/internal/token"
	"github.com/gamidoc/backend/internal/user"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidToken = errors.New("invalid or expired link")

const (
	verifyKind = "verify-email"
	resetKind  = "reset-password"

	verifyTTL     = 24 * time.Hour
	resetTTL      = time.Hour
	emailCooldown = time.Minute
	sendTimeout   = 15 * time.Second
)

// emailFlows sends the account emails (verification, password reset) and keeps their
// single-use tokens.
type emailFlows struct {
	mailer      mailer.Mailer
	tokens      *token.OneTimeStore
	fromEmail   string
	fromName    string
	frontendURL string
}

// WithEmail turns on email verification and password reset. Links point to
// frontendURL (?verify=... and ?reset=...).
func (s *Service) WithEmail(m mailer.Mailer, tokens *token.OneTimeStore, fromEmail, fromName, frontendURL string) *Service {
	s.email = &emailFlows{
		mailer:      m,
		tokens:      tokens,
		fromEmail:   fromEmail,
		fromName:    fromName,
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
	return s
}

// VerifyEmail confirms the address of the link's user and logs them in.
func (s *Service) VerifyEmail(ctx context.Context, tokenStr string) (AuthResult, error) {
	if s.email == nil {
		return AuthResult{}, ErrInvalidToken
	}
	userID, err := s.email.tokens.Consume(ctx, verifyKind, strings.TrimSpace(tokenStr))
	if err != nil {
		if errors.Is(err, token.ErrOneTimeTokenInvalid) {
			return AuthResult{}, ErrInvalidToken
		}
		return AuthResult{}, err
	}
	if err := s.users.MarkEmailVerified(ctx, userID); err != nil {
		return AuthResult{}, err
	}
	verified, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return AuthResult{}, err
	}
	return s.issueTokens(ctx, verified)
}

// ResendVerification sends the verification email again. It answers the same way
// whether the address exists or not, so it cannot be used to find accounts.
func (s *Service) ResendVerification(ctx context.Context, email string) error {
	if s.email == nil {
		return nil
	}
	found, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return nil
		}
		return err
	}
	if found.Verified() {
		return nil
	}
	if ok, err := s.email.tokens.Cooldown(ctx, verifyKind+":"+found.ID, emailCooldown); err != nil || !ok {
		return err
	}
	s.email.sendVerification(ctx, found)
	return nil
}

// ForgotPassword sends a password reset link. Same answer whether the address exists.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	if s.email == nil {
		return nil
	}
	found, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return nil
		}
		return err
	}
	if ok, err := s.email.tokens.Cooldown(ctx, resetKind+":"+found.ID, emailCooldown); err != nil || !ok {
		return err
	}
	link, err := s.email.link(ctx, resetKind, found.ID, resetTTL, "reset")
	if err != nil {
		return err
	}
	s.email.send(ctx, found.Email, "Reset your GamiDoc password",
		"We received a request to reset the password of your GamiDoc account.",
		"Choose a new password", link,
		"The link works once and expires in 1 hour. If you did not ask for it, ignore this email: your password stays the same.")
	return nil
}

// CheckResetToken reports whether a reset link is still valid (to show the form or an error).
func (s *Service) CheckResetToken(ctx context.Context, tokenStr string) error {
	if s.email == nil {
		return ErrInvalidToken
	}
	if _, err := s.email.tokens.Peek(ctx, resetKind, strings.TrimSpace(tokenStr)); err != nil {
		if errors.Is(err, token.ErrOneTimeTokenInvalid) {
			return ErrInvalidToken
		}
		return err
	}
	return nil
}

// ResetPassword sets a new password from a reset link and logs the user in. Opening the
// link also proves the address, so the account counts as verified.
func (s *Service) ResetPassword(ctx context.Context, tokenStr, password string) (AuthResult, error) {
	if s.email == nil {
		return AuthResult{}, ErrInvalidToken
	}
	if err := ValidatePassword(password); err != nil {
		return AuthResult{}, err
	}
	userID, err := s.email.tokens.Consume(ctx, resetKind, strings.TrimSpace(tokenStr))
	if err != nil {
		if errors.Is(err, token.ErrOneTimeTokenInvalid) {
			return AuthResult{}, ErrInvalidToken
		}
		return AuthResult{}, err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.users.UpdatePassword(ctx, userID, string(hashed)); err != nil {
		return AuthResult{}, err
	}
	if err := s.users.MarkEmailVerified(ctx, userID); err != nil {
		return AuthResult{}, err
	}
	updated, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return AuthResult{}, err
	}
	return s.issueTokens(ctx, updated)
}

func (e *emailFlows) sendVerification(ctx context.Context, u user.User) {
	link, err := e.link(ctx, verifyKind, u.ID, verifyTTL, "verify")
	if err != nil {
		slog.Error("verification link failed", "error", err)
		return
	}
	e.send(ctx, u.Email, "Confirm your GamiDoc email",
		"Welcome to GamiDoc! Confirm your email address to activate your account.",
		"Confirm my email", link,
		"The link expires in 24 hours. If you did not create a GamiDoc account, ignore this email.")
}

func (e *emailFlows) link(ctx context.Context, kind, userID string, ttl time.Duration, param string) (string, error) {
	tokenStr, err := e.tokens.Create(ctx, kind, userID, ttl)
	if err != nil {
		return "", err
	}
	return e.frontendURL + "/?" + param + "=" + url.QueryEscape(tokenStr), nil
}

// send delivers one account email. Errors are logged: the user can ask for it again.
func (e *emailFlows) send(ctx context.Context, to, subject, intro, button, link, outro string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sendTimeout)
	defer cancel()
	result, err := e.mailer.Send(ctx, mailer.Message{
		FromEmail: e.fromEmail,
		FromName:  e.fromName,
		To:        []string{to},
		Subject:   subject,
		Text:      fmt.Sprintf("%s\n\n%s: %s\n\n%s\n\nGamiDoc", intro, button, link, outro),
		HTML:      emailHTML(intro, button, link, outro),
	})
	if err != nil {
		slog.Error("account email not sent", "subject", subject, "error", err)
		return
	}
	if result.Provider == "noop" {
		// Local development without a mail provider: the link is in the log.
		slog.Info("account email (not sent, MAILER_PROVIDER=noop)", "to", to, "subject", subject, "link", link)
	}
}

func emailHTML(intro, button, link, outro string) string {
	l := html.EscapeString(link)
	return `<!doctype html><html><body style="margin:0;padding:24px;background:#f4f6fb;font-family:Arial,Helvetica,sans-serif;color:#1f2937">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td align="center">
<table role="presentation" width="100%" style="max-width:520px;background:#ffffff;border-radius:12px;padding:32px" cellspacing="0" cellpadding="0">
<tr><td style="font-size:22px;font-weight:bold;color:#1e3a8a;padding-bottom:16px">GamiDoc</td></tr>
<tr><td style="font-size:15px;line-height:1.6;padding-bottom:24px">` + html.EscapeString(intro) + `</td></tr>
<tr><td style="padding-bottom:24px"><a href="` + l + `" style="display:inline-block;background:#2563eb;color:#ffffff;text-decoration:none;font-weight:bold;padding:12px 24px;border-radius:8px">` + html.EscapeString(button) + `</a></td></tr>
<tr><td style="font-size:13px;line-height:1.6;color:#6b7280;padding-bottom:16px">` + html.EscapeString(outro) + `</td></tr>
<tr><td style="font-size:12px;color:#9ca3af;word-break:break-all">If the button does not work, copy this link into your browser:<br><a href="` + l + `" style="color:#2563eb">` + l + `</a></td></tr>
</table></td></tr></table></body></html>`
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
