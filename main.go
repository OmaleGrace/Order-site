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

	"golang.org/x/crypto/bcrypt"

	"github.com/joho/godotenv"
)

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

		err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password))
		if err != nil {
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

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Could not create account", http.StatusInternalServerError)
			return
		}

		_, err = db.Exec(
			"INSERT INTO users (name, email, password) VALUES ($1, $2, $3)",
			name,
			email,
			string(hashedPassword),
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

func naira(kobo int) int {
	return kobo / 100
}

func menuHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	tmpl := template.Must(
		template.New("menu.html").
			Funcs(template.FuncMap{
				"naira": naira,
			}).
			ParseFiles("templates/menu.html"),
	)

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

func addToCartHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	added, err := cart.AddItem(db, userID, itemID)
	if err != nil {
		fmt.Println("Add to cart error:", err)
		http.Error(w, "Could not add item to cart", http.StatusInternalServerError)
		return
	}

	if !added {
		http.Redirect(w, r, "/menu?message=Item%20already%20in%20cart", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/menu?message=Successfully%20added%20to%20cart", http.StatusSeeOther)

}

func cartHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	tmpl := template.Must(
		template.New("cart.html").
			Funcs(template.FuncMap{
				"naira": naira,
			}).
			ParseFiles("templates/cart.html"),
	)

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

	items, err := cart.GetItems(db, userID)
	if err != nil {
		fmt.Println("Cart load error:", err)
		http.Error(w, "Could not load cart", http.StatusInternalServerError)
		return
	}

	var total int
	for _, item := range items {
		total += item.PriceKobo * item.Quantity
	}

	data := struct {
		Items []cart.CartItem
		Total int
	}{
		Items: items,
		Total: total,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Cart template error:", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func removeFromCartHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	err = cart.RemoveItem(db, userID, itemID)
	if err != nil {
		fmt.Println("Remove from cart error:", err)
	}

	http.Redirect(w, r, "/cart", http.StatusSeeOther)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found, using system environment")
	}

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	db, err := database.Connect()
	if err != nil {
		fmt.Println("Database connection failed:", err)
		return
	}
	defer db.Close()

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		loginHandler(w, r, db)
	})
	http.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		signupHandler(w, r, db)
	})
	http.HandleFunc("/menu", func(w http.ResponseWriter, r *http.Request) {
		menuHandler(w, r, db)
	})
	http.HandleFunc("/cart/add", func(w http.ResponseWriter, r *http.Request) {
		addToCartHandler(w, r, db)
	})
	http.HandleFunc("/cart", func(w http.ResponseWriter, r *http.Request) {
		cartHandler(w, r, db)
	})
	http.HandleFunc("/cart/remove", func(w http.ResponseWriter, r *http.Request) {
		removeFromCartHandler(w, r, db)
	})

	fmt.Println("Server Running on https://localhost:8080")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}
