package handlers

import (
	"strings"
	"testing"
)

func TestPasswordLengthCap(t *testing.T) {
	if validatePassword("Aa1!"+strings.Repeat("x", 80)) == nil {
		t.Error("passwords over 72 bytes must be refused")
	}
	if err := validatePassword("Str0ng!Pass"); err != nil {
		t.Error(err)
	}
}

func TestValidImageURL(t *testing.T) {
	good := []string{"", "https://res.cloudinary.com/x/y.jpg"}
	bad := []string{"http://example.com/a.jpg", "javascript:alert(1)", "data:text/html,hi", "//evil.com/a.jpg", "https://user:pw@evil.com/a.jpg", "https://" + strings.Repeat("a", 600) + ".com"}
	for _, g := range good {
		if m := validImageURL(g); m != "" {
			t.Errorf("%q refused: %s", g, m)
		}
	}
	for _, b := range bad {
		if validImageURL(b) == "" {
			t.Errorf("%q should be refused", b)
		}
	}
}
