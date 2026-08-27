package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"Order-site/cart"
)

func (h *Handlers) AddToCart(w http.ResponseWriter, r *http.Request) {
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

func (h *Handlers) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
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

	err = cart.RemoveItem(h.DB, userID, itemID)
	if err != nil {
		fmt.Println("Remove from cart error:", err)
	}

	http.Redirect(w, r, "/cart", http.StatusSeeOther)
}