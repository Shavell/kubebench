// Package password holds the password policy (docs/design-v0.5.md §3.5),
// shared by the API and the bootstrap admin's configuration.
package password

import (
	"unicode/utf8"

	"kubebench/internal/reason"
)

const (
	// MinLength is the minimum length in characters.
	MinLength = 8
	// MaxBytes is the most bcrypt reads; it rejects longer passwords.
	MaxBytes = 72
)

// Reasons of Check (§3.3).
var (
	reasonTooShort = reason.Define("password.too_short", "the password must be at least {{min}} characters")
	reasonTooLong  = reason.Define("password.too_long", "the password must be at most {{max}} bytes")
)

// Check explains why pw does not meet the policy ("the password must be
// at least 8 characters", a *reason.Error), or returns nil.
func Check(pw string) error {
	if utf8.RuneCountInString(pw) < MinLength {
		return reason.New(reasonTooShort, reason.Params{"min": MinLength})
	}
	if len(pw) > MaxBytes {
		return reason.New(reasonTooLong, reason.Params{"max": MaxBytes})
	}
	return nil
}
