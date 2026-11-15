package auth

import "unicode"

// Password rules, shown to the user in the sign-up form (keep them in sync with
// frontend_new/src/utils/passwordRules.js).
const (
	PasswordMinLength = 8
	// bcrypt ignores everything after 72 bytes.
	PasswordMaxLength = 72
)

// PasswordRulesText is the error message listing the rules.
const PasswordRulesText = "Password must be 8-72 characters long and contain an uppercase letter, a lowercase letter and a number"

// ValidatePassword checks the password rules.
func ValidatePassword(password string) error {
	if len(password) < PasswordMinLength || len(password) > PasswordMaxLength {
		return ErrInvalidPassword
	}
	var upper, lower, digit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if !upper || !lower || !digit {
		return ErrInvalidPassword
	}
	return nil
}
