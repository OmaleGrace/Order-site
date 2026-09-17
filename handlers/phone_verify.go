package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

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

	_, err = middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	phone := strings.TrimSpace(r.FormValue("phone"))

if phone == "" {
	errors.Render(w, "Phone number is required", http.StatusBadRequest)
	return
}

phone = normalizePhone(phone)

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

	tmpl, err := template.ParseFiles("templates/verify-phone.html")
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

	phone := strings.TrimSpace(r.FormValue("phone"))
	pinID := r.FormValue("pin_id")
	code := strings.TrimSpace(r.FormValue("code"))

	if phone == "" || pinID == "" || code == "" {
		errors.Render(w, "Missing verification details", http.StatusBadRequest)
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