package main

import (
	"Order-site/cart"
	"Order-site/database"
	"Order-site/menu"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"github.com/joho/godotenv"
)

var carts = make(map[int]*cart.Cart)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/home.html"))

	err := tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Something went Wrong", http.StatusInternalServerError)
		return
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

		err = db.QueryRow(
			"SELECT id, password FROM users WHERE email = $1",
			email,
		).Scan(&userID, &storedPassword)

		if err != nil {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		if password != storedPassword {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "user_id",
			Value: fmt.Sprintf("%d", userID),
			Path:  "/",
		})
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}

	err := tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func signupHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

		_, err = db.Exec(
			"INSERT INTO users (name, email, password) VALUES ($1, $2, $3)",
			name,
			email,
			password,
		)
		if err != nil {
			http.Error(w, "Could not create account", http.StatusInternalServerError)
			return
		}
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

func menuHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	tmpl := template.Must(template.ParseFiles("templates/menu.html"))

	items, err := menu.GetAll(db)
	if err != nil {
		fmt.Println("Menu load error", err)
		http.Error(w, "Could not load menu", http.StatusInternalServerError)
		return
	}

	message := r.URL.Query().Get("message")

	data := struct {
		Items   []menu.MenuItem
		Message string
	}{
		Items:   items,
		Message: message,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Template error:", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func addToCartHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("user_id")
	if err != nil {
		http.Error(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	itemID, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "invalid item ID", http.StatusBadRequest)
		return
	}

	if carts[userID] == nil {
		carts[userID] = &cart.Cart{}
	}

	userCart := carts[userID]
	fmt.Println("User:", userID, "Current cart:", userCart.Items)
	for _, id := range userCart.Items {
		if id == itemID {
			http.Redirect(w, r, "/menu?message=Item%20already%20in%20cart", http.StatusSeeOther)
			return
		}
	}

	userCart.Add(itemID)
	http.Redirect(w, r, "/menu?message=Successfully%20added%20to%20cart", http.StatusSeeOther)
}

func cartHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/cart.html"))

	cookie, err := r.Cookie("user_id")
	if err != nil {
		http.Error(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	userCart := carts[userID]
	if userCart == nil {
		userCart = &cart.Cart{}
	}

	err = tmpl.Execute(w, userCart)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found, using system environment")
	}

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static", http.StripPrefix("/static/", fs))

	db, err := database.Connect()
	if err != nil {
		fmt.Println("Database connection failed:", err)
		return
	}
	defer db.Close()

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/cart/add", addToCartHandler)
	http.HandleFunc("/cart", cartHandler)
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		loginHandler(w, r, db)
	})
	http.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		signupHandler(w, r, db)
	})

	http.HandleFunc("/menu", func(w http.ResponseWriter, r *http.Request) {
		menuHandler(w, r, db)
	})

	fmt.Println("Server Running on https://localhost:8080")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}
