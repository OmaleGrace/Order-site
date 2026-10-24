package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/sms"
)

func (h *Handlers) SendPhoneOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	phone := strings.TrimSpace(r.FormValue("phone"))

	if phone == "" {
		errors.Render(w, "Phone number is required", http.StatusBadRequest)
		return
	}

	// Each text message costs money, so sending is limited per person and per address
	uid := strconv.Itoa(userID)
	if !middleware.OTPSendUser.Hit(uid) || !middleware.OTPSendIP.Hit(middleware.ClientIP(r)) {
		errors.Render(w, "Too many codes requested. Please try again in an hour.", http.StatusTooManyRequests)
		return
	}

	phone = normalizePhone(phone)
	if normalizeNigerianPhone(phone) == "" {
		errors.Render(w, "Enter a valid Nigerian phone number: 11 digits starting with 0 (for example 08012345678), or starting with +234.", http.StatusBadRequest)
		return
	}

	// Make sure this phone isn't already used by another account
	var existingCount int
	err = h.DB.QueryRow("SELECT COUNT(*) FROM users WHERE phone = $1", phone).Scan(&existingCount)
	if err != nil {
		errors.Render(w, "Could not verify phone number", http.StatusInternalServerError)
		return
	}

	if existingCount > 0 {
		errors.Render(w, "This phone number is already registered to another account", http.StatusBadRequest)
		return
	}

	pinID, err := sms.SendOTP(phone)
	if err != nil {
		fmt.Println("Send OTP error:", err)
		errors.Render(w, "Could not send verification code. Please check the number and try again.", http.StatusInternalServerError)
		return
	}

	// Remember who this code was sent to, and which number
	if _, err := h.DB.Exec(`
		INSERT INTO phone_verifications (pin_id, user_id, phone)
		VALUES ($1, $2, $3)
		ON CONFLICT (pin_id) DO UPDATE SET user_id = EXCLUDED.user_id, phone = EXCLUDED.phone, created_at = NOW()
	`, pinID, userID, phone); err != nil {
		fmt.Println("Save phone verification error:", err)
		errors.Render(w, "Could not send verification code. Please try again.", http.StatusInternalServerError)
		return
	}

	tmpl, err := brand.ParseE("templates/verify-phone.html")
	if err != nil {
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	data := struct {
		Phone string
		PinID string
	}{
		Phone: phone,
		PinID: pinID,
	}

	tmpl.Execute(w, data)
}

func (h *Handlers) VerifyPhoneOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pinID := strings.TrimSpace(r.FormValue("pin_id"))
	code := strings.TrimSpace(r.FormValue("code"))

	if pinID == "" || code == "" {
		errors.Render(w, "Missing verification details", http.StatusBadRequest)
		return
	}

	if !middleware.OTPVerifyUser.Hit(strconv.Itoa(userID)) {
		errors.Render(w, "Too many attempts. Please try again in an hour.", http.StatusTooManyRequests)
		return
	}

	// The number to save comes from our own record of where the code was sent,
	// never from the form. A code only verifies the number it was sent to,
	// and only for the person it was sent to.
	var phone string
	err = h.DB.QueryRow(`
		SELECT phone FROM phone_verifications
		WHERE pin_id = $1 AND user_id = $2 AND created_at > NOW() - INTERVAL '15 minutes'
	`, pinID, userID).Scan(&phone)
	if err != nil {
		errors.Render(w, "That code has expired or was not sent to you. Please request a new one.", http.StatusBadRequest)
		return
	}

	verified, err := sms.VerifyOTP(pinID, code)
	if err != nil {
		errors.Render(w, "Could not verify code. Please try again.", http.StatusInternalServerError)
		return
	}

	if !verified {
		errors.Render(w, "Incorrect or expired code. Please request a new one.", http.StatusBadRequest)
		return
	}

	h.DB.Exec("DELETE FROM phone_verifications WHERE pin_id = $1", pinID)

	_, err = h.DB.Exec("UPDATE users SET phone = $1 WHERE id = $2", phone, userID)
	if err != nil {
		errors.Render(w, "Phone verified, but could not save it. It may already be registered to another account.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/account?updated=true", http.StatusSeeOther)
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	if strings.HasPrefix(phone, "+234") {
		return phone[1:]
	}
	if strings.HasPrefix(phone, "234") {
		return phone
	}
	if strings.HasPrefix(phone, "0") {
		return "234" + phone[1:]
	}
	return phone
}
