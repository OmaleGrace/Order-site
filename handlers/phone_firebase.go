package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Order-site/firebaseauth"
	"Order-site/middleware"
)

// verifyFirebaseToken is a variable so tests can replace it.
var verifyFirebaseToken = firebaseauth.VerifyPhoneToken

// maxFirebaseProofAge is how long ago the visitor may have typed the code.
const maxFirebaseProofAge = 10 * time.Minute

// newFirebaseVerificationID makes the private id that ties a Firebase check to one person and
// one phone number. The "fb_" prefix tells it apart from the older Termii codes.
func newFirebaseVerificationID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "fb_" + hex.EncodeToString(b), nil
}

// checkFirebaseProof decides whether a verified Firebase sign-in proves the phone number we
// sent the code to. expectedPhone is the number saved when the code was requested
// (format 2348012345678).
func checkFirebaseProof(proof firebaseauth.Proof, expectedPhone string, now time.Time) error {
	if normalizePhone(proof.Phone) != expectedPhone {
		return errors.New("verified number does not match the number requested")
	}
	if now.Sub(proof.AuthTime) > maxFirebaseProofAge || proof.AuthTime.After(now.Add(time.Minute)) {
		return errors.New("verification is too old")
	}
	return nil
}

type firebaseVerifyRequest struct {
	PinID   string `json:"pin_id"`
	IDToken string `json:"id_token"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// FirebaseVerifyPhone is called by the browser after the visitor typed the code Firebase
// texted them. It checks the proof with the Firebase Admin SDK and then saves the number.
func (h *Handlers) FirebaseVerifyPhone(w http.ResponseWriter, r *http.Request) {
	userID, err := requestUserID(h, r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Please log in again."})
		return
	}
	if !firebaseauth.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Phone verification is not available right now."})
		return
	}

	var req firebaseVerifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil ||
		!strings.HasPrefix(req.PinID, "fb_") || req.IDToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing verification details."})
		return
	}

	if !middleware.OTPVerifyUser.Hit(strconv.Itoa(userID)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many attempts. Please try again in an hour."})
		return
	}

	// The number to save is the one we recorded when this person asked for the code,
	// never one sent by the browser.
	var phone string
	err = h.DB.QueryRow(`
		SELECT phone FROM phone_verifications
		WHERE pin_id = $1 AND user_id = $2 AND created_at > NOW() - INTERVAL '15 minutes'
	`, req.PinID, userID).Scan(&phone)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "This verification expired. Please start again."})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	proof, err := verifyFirebaseToken(ctx, req.IDToken)
	if err != nil {
		log.Printf("firebase phone verify (user %d): %v", userID, err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "We could not confirm that code. Please try again."})
		return
	}
	if err := checkFirebaseProof(proof, phone, time.Now()); err != nil {
		log.Printf("firebase phone verify (user %d): %v", userID, err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "We could not confirm that number. Please start again."})
		return
	}

	h.DB.Exec("DELETE FROM phone_verifications WHERE pin_id = $1", req.PinID)
	if _, err := h.DB.Exec("UPDATE users SET phone = $1 WHERE id = $2", phone, userID); err != nil {
		log.Printf("firebase phone verify (user %d): save failed: %v", userID, err)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Phone verified, but could not save it. It may already be registered to another account."})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"redirect": "/account?updated=true"})
}
