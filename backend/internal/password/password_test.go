package password

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		pw string
		ok bool
	}{
		{"1234567", false},
		{"12345678", true},
		{"ğüşöçıİ€", true}, // 8 characters, more than 8 bytes
		{strings.Repeat("a", MaxBytes), true},
		{strings.Repeat("a", MaxBytes+1), false},
		{strings.Repeat("ş", MaxBytes/2+1), false}, // 37 characters, 74 bytes
	} {
		if err := Check(tc.pw); (err == nil) != tc.ok {
			t.Errorf("Check(%q) = %v, want ok=%v", tc.pw, err, tc.ok)
		}
	}
}
