package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	emailer "Order-site/email"
	"Order-site/errors"

	"golang.org/x/crypto/bcrypt"
)

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *Handlers) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/forgot-password.html"))

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			errors.Render(w, "Something went wrong", http.StatusBadRequest)
			return
		}

		email := r.FormValue("email")

		var userID int
		err = h.DB.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)

		// Always show the same message, whether or not the email exists —
		// this avoids revealing which emails are registered.
		if err == nil {
			token, err := generateToken()
			if err != nil {
				fmt.Println("Generate token error:", err)
				errors.Render(w, "Something went wrong", http.StatusInternalServerError)
				return
			}

			_, err = h.DB.Exec(`
				INSERT INTO password_reset_tokens (token, user_id, expires_at)
				VALUES ($1, $2, NOW() + INTERVAL '15 minutes')
			`, token, userID)

			if err != nil {
				fmt.Println("Save reset token error:", err)
				errors.Render(w, "Something went wrong", http.StatusInternalServerError)
				return
			}

			resetLink := fmt.Sprintf("%s/reset-password?token=%s", os.Getenv("APP_BASE_URL"), token)

			go func() {
				if err := emailer.Send(email, "Reset Your Password", fmt.Sprintf("<h1>Reset your password</h1><p>Click the link below to reset your password. This link expires in 15 minutes.</p><p><a href=\"%s\">Reset Password</a></p>", resetLink)); err != nil {
					fmt.Println("Password reset email error:", err)
				}
			}()
		}

		fmt.Fprint(w, `
    <html>
        <body>
            <h1>Check your email</h1>
            <p>If an account exists with that email, a password reset link has been sent.</p>
            <a href="/login">Back to Login</a>
        </body>
    </html>
`)
		return
	}

	err := tmpl.Execute(w, nil)
	if err != nil {
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/reset-password.html"))

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			errors.Render(w, "Something went wrong", http.StatusBadRequest)
			return
		}

		token := r.FormValue("token")
		newPassword := r.FormValue("password")

		var userID int
		var expiresAt time.Time

		err = h.DB.QueryRow(`
			SELECT user_id, expires_at FROM password_reset_tokens WHERE token = $1
		`, token).Scan(&userID, &expiresAt)

		if err != nil {
			errors.Render(w, "This reset link is invalid or has already been used", http.StatusBadRequest)
			return
		}

		if time.Now().After(expiresAt) {
			errors.Render(w, "This reset link has expired", http.StatusBadRequest)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			errors.Render(w, "Could not reset password", http.StatusInternalServerError)
			return
		}

		_, err = h.DB.Exec("UPDATE users SET password = $1 WHERE id = $2", string(hashedPassword), userID)
		if err != nil {
			fmt.Println("Update password error:", err)
			errors.Render(w, "Could not reset password", http.StatusInternalServerError)
			return
		}

		// Token is single-use — delete it now that it's been used
		_, _ = h.DB.Exec("DELETE FROM password_reset_tokens WHERE token = $1", token)

		fmt.Fprint(w, `
    <html>
        <body>
            <h1>Password reset successfully!</h1>
            <p>Redirecting to login...</p>

            <script>
                setTimeout(function() {
                    window.location.href = "/login";
                }, 1500);
            </script>
        </body>
    </html>
`)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		errors.Render(w, "Missing reset token", http.StatusBadRequest)
		return
	}

	data := struct {
		Token string
	}{
		Token: token,
	}

	err := tmpl.Execute(w, data)
	if err != nil {
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}
