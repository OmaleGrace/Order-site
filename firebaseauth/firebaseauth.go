// Package firebaseauth checks sign-in proof from Firebase on the server.
//
// Firebase sends the verification text message from the visitor's browser (see
// static/js/phone-otp.js). The browser then hands our server a short-lived "ID token".
// This package asks the Firebase Admin SDK whether that token is genuine and which phone
// number it proves. The Admin SDK cannot send text messages itself.
package firebaseauth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// Proof is what a verified phone sign-in tells us.
type Proof struct {
	Phone    string    // in +E.164 form, for example +2348012345678
	AuthTime time.Time // when the visitor typed the code
}

var (
	once    sync.Once
	client  *auth.Client
	initErr error
)

// Enabled reports whether Firebase credentials are configured. When it is false the site
// falls back to the older Termii text messages.
func Enabled() bool {
	return strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")) != "" ||
		strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_FILE")) != ""
}

func authClient(ctx context.Context) (*auth.Client, error) {
	once.Do(func() {
		var opt option.ClientOption
		if raw := strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")); raw != "" {
			opt = option.WithCredentialsJSON([]byte(raw))
		} else if path := strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_FILE")); path != "" {
			opt = option.WithCredentialsFile(path)
		} else {
			initErr = errors.New("firebase is not configured")
			return
		}
		app, err := firebase.NewApp(ctx, nil, opt)
		if err != nil {
			initErr = fmt.Errorf("firebase app: %w", err)
			return
		}
		client, initErr = app.Auth(ctx)
	})
	return client, initErr
}

// VerifyPhoneToken checks an ID token from the browser. It succeeds only for a genuine,
// unexpired token that came from signing in with a phone number.
func VerifyPhoneToken(ctx context.Context, idToken string) (Proof, error) {
	c, err := authClient(ctx)
	if err != nil {
		return Proof{}, err
	}
	tok, err := c.VerifyIDToken(ctx, idToken)
	if err != nil {
		return Proof{}, fmt.Errorf("verify id token: %w", err)
	}
	if tok.Firebase.SignInProvider != "phone" {
		return Proof{}, errors.New("token is not from a phone sign-in")
	}
	phone, _ := tok.Claims["phone_number"].(string)
	if phone == "" {
		return Proof{}, errors.New("token has no phone number")
	}
	return Proof{Phone: phone, AuthTime: time.Unix(tok.AuthTime, 0)}, nil
}
