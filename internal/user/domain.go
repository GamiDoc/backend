package user

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	// EmailVerifiedAt is nil until the user opens the link of the verification email.
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
}

// Verified reports whether the user confirmed their email address.
func (u User) Verified() bool {
	return u.EmailVerifiedAt != nil
}
