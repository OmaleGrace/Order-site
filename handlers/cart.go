package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"Order-site/cart"
	"Order-site/middleware"
)

func (h *Handlers) AddToCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	itemID, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "invalid item ID", http.StatusBadRequest)
		return
	}

	added, err := cart.AddItem(h.DB, userID, itemID)
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

func (h *Handlers) Cart(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(
		template.New("cart.html").
			Funcs(template.FuncMap{
	"naira": naira,
	"multiply": func(price, quantity int) int {
		return price * quantity
	},
}).
			ParseFiles("templates/cart.html"),
	)

	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	items, err := cart.GetItems(h.DB, userID)
	if err != nil {
		fmt.Println("Cart load error:", err)
		http.Error(w, "Could not load cart", http.StatusInternalServerError)
		return
	}

	var total int
	for _, item := range items {
		total += item.PriceKobo * item.Quantity
	}

	message := r.URL.Query().Get("message")

	data := struct {
		Items []cart.CartItem
		Total int
		Message string
	}{
		Items: items,
		Total: total,
		Message: message,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Cart template error:", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	itemID, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	err = cart.RemoveItem(h.DB, userID, itemID)
	if err != nil {
		fmt.Println("Remove from cart error:", err)
	}

	http.Redirect(w, r, "/cart", http.StatusSeeOther)
}

func (h *Handlers) UpdateCartQuantity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	itemID, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")

	switch action {
	case "increase":
		_, err = h.DB.Exec(`
			UPDATE cart_items
			SET quantity = quantity + 1
			WHERE user_id = $1 AND menu_item_id = $2
		`, userID, itemID)

	case "decrease":
	var quantity int

	err = h.DB.QueryRow(`
		SELECT quantity
		FROM cart_items
		WHERE user_id = $1 AND menu_item_id = $2
	`, userID, itemID).Scan(&quantity)

	if err != nil {
		http.Error(w, "Could not update cart", http.StatusInternalServerError)
		return
	}

	if quantity <= 1 {
		http.Redirect(
			w,
			r,
			"/cart?message=Minimum%20order%20quantity%20is%201",
			http.StatusSeeOther,
		)
		return
	}

	_, err = h.DB.Exec(`
		UPDATE cart_items
		SET quantity = quantity - 1
		WHERE user_id = $1 AND menu_item_id = $2
	`, userID, itemID)

	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	if err != nil {
		fmt.Println("Update cart quantity error:", err)
		http.Error(w, "Could not update cart", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cart", http.StatusSeeOther)
}
