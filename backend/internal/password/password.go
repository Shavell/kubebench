// Package password holds the password policy (docs/design-v0.5.md §3.5),
// shared by the API and the bootstrap admin's configuration.
package password

import (
	"fmt"
	"unicode/utf8"
)

const (
	// MinLength is the minimum length in characters.
	MinLength = 8
	// MaxBytes is the most bcrypt reads; it rejects longer passwords.
	MaxBytes = 72
)

// Check explains why pw does not meet the policy ("must be at least 8
// characters"), or returns nil.
func Check(pw string) error {
	if utf8.RuneCountInString(pw) < MinLength {
		return fmt.Errorf("must be at least %d characters", MinLength)
	}
	if len(pw) > MaxBytes {
		return fmt.Errorf("must be at most %d bytes", MaxBytes)
	}
	return nil
}
