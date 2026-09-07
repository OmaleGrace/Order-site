package handlers

import (
	"fmt"
	"html/template"
	"net/http"

	"Order-site/middleware"

	emailer "Order-site/email"

	"golang.org/x/crypto/bcrypt"
)

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/login.html"))

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Something went wrong", http.StatusBadRequest)
			return
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		var userID int
		var storedPassword string
		var isAdmin bool

		err = h.DB.QueryRow(
			"SELECT id, password, is_admin FROM users WHERE email = $1",
			email,
		).Scan(&userID, &storedPassword, &isAdmin)

		if err != nil {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		err = bcrypt.CompareHashAndPassword(
			[]byte(storedPassword),
			[]byte(password),
		)

		if err != nil {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		// Create a secure server-side session
		token, err := middleware.CreateSession(h.DB, userID)
		if err != nil {
			http.Error(w, "Could not create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   60 * 60 * 24,
		})

		// Automatically redirect admins to the admin dashboard
		if isAdmin {
			http.Redirect(w, r, "/admin/orders", http.StatusSeeOther)
			return
		}

		// Normal users go to the menu
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}

	err := tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) Signup(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/signup.html"))

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Something went wrong", http.StatusBadRequest)
			return
		}

		name := r.FormValue("name")
		email := r.FormValue("email")
		password := r.FormValue("password")

		hashedPassword, err := bcrypt.GenerateFromPassword(
			[]byte(password),
			bcrypt.DefaultCost,
		)
		if err != nil {
			http.Error(w, "Could not create account", http.StatusInternalServerError)
			return
		}

		_, err = h.DB.Exec(
			"INSERT INTO users (name, email, password) VALUES ($1, $2, $3)",
			name,
			email,
			string(hashedPassword),
		)
		if err != nil {
			http.Error(w, "Could not create account", http.StatusInternalServerError)
			return
		}

		go emailer.Send(
			email,
			"Welcome to Grace's Kitchen!",
			fmt.Sprintf("<h1>Welcome, %s!</h1><p>Your account has been created successfully. Start browsing our menu and place your first order today.</p>", name),
		)

		fmt.Fprint(w, `
    <html>
        <body>
            <h1>Account created successfully!</h1>
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

	err := tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}
