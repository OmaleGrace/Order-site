package handlers

import (
	"strings"
	"testing"
	"time"

	"Order-site/firebaseauth"
)

func TestCheckFirebaseProof(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	good := firebaseauth.Proof{Phone: "+2348012345678", AuthTime: now.Add(-time.Minute)}

	if err := checkFirebaseProof(good, "2348012345678", now); err != nil {
		t.Errorf("a fresh proof for the right number should pass: %v", err)
	}
	if err := checkFirebaseProof(good, "2348099999999", now); err == nil {
		t.Error("a proof for a different number must be rejected")
	}
	old := firebaseauth.Proof{Phone: "+2348012345678", AuthTime: now.Add(-time.Hour)}
	if err := checkFirebaseProof(old, "2348012345678", now); err == nil {
		t.Error("an old proof must be rejected")
	}
	future := firebaseauth.Proof{Phone: "+2348012345678", AuthTime: now.Add(time.Hour)}
	if err := checkFirebaseProof(future, "2348012345678", now); err == nil {
		t.Error("a proof dated in the future must be rejected")
	}
}

func TestNewFirebaseVerificationID(t *testing.T) {
	a, err := newFirebaseVerificationID()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := newFirebaseVerificationID()
	if !strings.HasPrefix(a, "fb_") || len(a) != 35 || a == b {
		t.Errorf("unexpected ids %q and %q", a, b)
	}
}
