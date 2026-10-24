package handlers

import (
	"strings"
	"testing"
)

func TestValidateChatMessage(t *testing.T) {
	cases := []struct {
		name, input, want string
		valid             bool
	}{
		{"normal", " Hello vendor ", "Hello vendor", true},
		{"empty", "  ", "", false},
		{"too long", strings.Repeat("a", 2001), "", false},
		{"at limit", strings.Repeat("a", 2000), strings.Repeat("a", 2000), true},
		{"null byte removed", "hello\x00world", "helloworld", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := validateChatMessage(tc.input)
			if got != tc.want || ok != tc.valid {
				t.Fatalf("validateChatMessage() = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.valid)
			}
		})
	}
}
