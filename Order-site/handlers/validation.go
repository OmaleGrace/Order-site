package handlers

import (
	"errors"
	"net/mail"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validateName(name string) error {
	words := strings.Fields(name)
	if len(words) < 2 {
		return errors.New("Enter your full name, with words separated by a space")
	}
	for _, w := range words {
		if utf8.RuneCountInString(w) < 3 {
			return errors.New("Each part of your name must be at least 3 letters")
		}
	}
	return nil
}

func validateEmail(email string) error {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return errors.New("Enter a valid email address")
	}
	at := strings.LastIndex(email, "@")
	if !strings.Contains(email[at+1:], ".") {
		return errors.New("Enter a valid email address")
	}
	return nil
}

func validatePassword(p string) error {
	if utf8.RuneCountInString(p) < 8 {
		return errors.New("Password must be at least 8 characters")
	}
	// bcrypt only uses the first 72 bytes, so longer passwords are refused
	if len(p) > 72 {
		return errors.New("Password must be 72 characters or fewer")
	}
	var upper, lower, digit, special bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			special = true
		}
	}
	if !upper || !lower || !digit || !special {
		return errors.New("Password needs a capital letter, a lowercase letter, a number and a special character")
	}
	return nil
}

// validImageURL accepts only secure web addresses for pictures.
// It returns an empty string when the address is fine, otherwise the problem.
func validImageURL(raw string) string {
	if raw == "" {
		return ""
	}
	if len(raw) > 500 {
		return "The picture link is too long"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "The picture link must start with https://"
	}
	return ""
}
