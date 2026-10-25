package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"

	"Order-site/brand"
	emailer "Order-site/email"
	"Order-site/errors"
	"Order-site/middleware"

	"golang.org/x/crypto/bcrypt"
)

// dummyHash is a real bcrypt hash of a random string. It is compared against when
// no matching account exists, so a wrong email and a wrong password take equally long.
var dummyHash = func() string {
	secret, _ := generateToken()
	h, _ := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	return string(h)
}()

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	tmpl := brand.Parse("templates/login.html")

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			errors.Render(w, "Something went wrong", http.StatusBadRequest)
			return
		}

		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		password := r.FormValue("password")

		// Too many wrong guesses for this email, or from this address: stop and wait.
		// The same message is used whether or not the email exists.
		ip := middleware.ClientIP(r)
		if middleware.LoginFailsByEmail.Blocked(email) {
			log.Printf("login: locked out email attempt from ip=%s", ip)
			w.Header().Set("Retry-After", "900")
			errors.Render(w, "Too many failed attempts. Please wait 15 minutes, or reset your password.", http.StatusTooManyRequests)
			return
		}

		var userID int
		var storedPassword sql.NullString
		var isAdmin bool

		err = h.DB.QueryRow(
			"SELECT id, password, is_admin FROM users WHERE LOWER(email) = $1",
			email,
		).Scan(&userID, &storedPassword, &isAdmin)

		// Always do the slow password check, even when the account does not exist
		// or has no password (Google sign-in), so response time does not reveal which.
		hashToCheck := dummyHash
		if err == nil && storedPassword.Valid && storedPassword.String != "" {
			hashToCheck = storedPassword.String
		}
		cmpErr := bcrypt.CompareHashAndPassword([]byte(hashToCheck), []byte(password))

		if err != nil || !storedPassword.Valid || cmpErr != nil {
			middleware.LoginFailsByEmail.Hit(email)
			errors.Render(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		middleware.LoginFailsByEmail.Reset(email)

		// Create a secure server-side session
		token, err := middleware.CreateSession(h.DB, userID)
		if err != nil {
			errors.Render(w, "Could not create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   middleware.SecureCookie(r),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   60 * 60 * 24,
		})

		// Automatically redirect admins to the admin dashboard
		if isAdmin {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}

		// Normal users go to the menu
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}

	err := tmpl.Execute(w, nil)
	if err != nil {
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

func signupFail(w http.ResponseWriter, r *http.Request, msg string, code int) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": msg})
		return
	}
	errors.Render(w, msg, code)
}

func (h *Handlers) Signup(w http.ResponseWriter, r *http.Request) {
	tmpl := brand.Parse("templates/signup.html")

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			signupFail(w, r, "Something went wrong", http.StatusBadRequest)
			return
		}

		firstName := strings.TrimSpace(r.FormValue("first_name"))
		middleName := strings.TrimSpace(r.FormValue("middle_name"))
		lastName := strings.TrimSpace(r.FormValue("last_name"))
		nameParts := []string{firstName}
		if middleName != "" {
			nameParts = append(nameParts, middleName)
		}
		nameParts = append(nameParts, lastName)
		name := strings.Join(nameParts, " ")
		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		phone := normalizePhone(strings.TrimSpace(r.FormValue("phone")))
		country := strings.TrimSpace(r.FormValue("country"))
		state := strings.TrimSpace(r.FormValue("state"))
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		termsAccepted := r.FormValue("terms_accepted") == "on"

		if err := validateNamePart(firstName, "First name", true); err != nil {
			signupFail(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateNamePart(middleName, "Middle name", false); err != nil {
			signupFail(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateNamePart(lastName, "Last name", true); err != nil {
			signupFail(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateEmail(email); err != nil {
			signupFail(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if phone == "" || normalizeNigerianPhone(phone) == "" {
			signupFail(w, r, "Enter a valid Nigerian phone number, for example 08012345678 or +2348012345678", http.StatusBadRequest)
			return
		}
		if country != "Nigeria" {
			signupFail(w, r, "Please select Nigeria as your country", http.StatusBadRequest)
			return
		}
		if !validState(state) {
			signupFail(w, r, "Select a valid Nigerian state", http.StatusBadRequest)
			return
		}
		if err := validatePassword(password); err != nil {
			signupFail(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if password != confirmPassword {
			signupFail(w, r, "Passwords do not match", http.StatusBadRequest)
			return
		}
		if !termsAccepted {
			signupFail(w, r, "Please read and accept the Terms of Service and Privacy Policy before creating your account", http.StatusBadRequest)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			signupFail(w, r, "Could not create account", http.StatusInternalServerError)
			return
		}

		var existing bool
		if err := h.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email) = $1)", email).Scan(&existing); err == nil && existing {
			signupFail(w, r, "An account with this email already exists. Try logging in instead.", http.StatusConflict)
			return
		}

		var userID int
		err = h.DB.QueryRow(
			`INSERT INTO users (name, first_name, middle_name, last_name, email, password, phone, country, state, terms_accepted_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW()) RETURNING id`,
			name, firstName, middleName, lastName, email, string(hashedPassword), phone, country, state,
		).Scan(&userID)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				signupFail(w, r, "An account with this email already exists. Try logging in instead.", http.StatusConflict)
				return
			}
			fmt.Println("Signup insert error:", err)
			signupFail(w, r, "Could not create account", http.StatusInternalServerError)
			return
		}

		// Log the new customer in straight away
		token, err := middleware.CreateSession(h.DB, userID)
		if err != nil {
			fmt.Println("Signup session error:", err)
			signupFail(w, r, "Account created, but we could not log you in. Please log in.", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   middleware.SecureCookie(r),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   60 * 60 * 24,
		})

		go func() {
			if err := emailer.Send(email, "Welcome to "+brand.Name()+"!", fmt.Sprintf("<h1>Welcome, %s!</h1><p>Your account has been created successfully. Start browsing our menu and place your first order today.</p>", html.EscapeString(firstName))); err != nil {
				fmt.Println("Welcome email error:", err)
			}
		}()

		if wantsJSON(r) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "redirect": "/menu"})
			return
		}
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}
