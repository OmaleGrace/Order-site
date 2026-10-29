package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Order-site/cart"
	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

func (h *Handlers) AddToCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err != nil {
		errors.Render(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		errors.Render(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	itemID, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		errors.Render(w, "invalid item ID", http.StatusBadRequest)
		return
	}

	var available bool
	if err := h.DB.QueryRow(`SELECT COALESCE(available, TRUE) FROM menu_items WHERE id = $1`, itemID).Scan(&available); err != nil {
		errors.Render(w, "Could not find that menu item", http.StatusBadRequest)
		return
	}
	if !available {
		errors.Render(w, "This item is currently unavailable", http.StatusBadRequest)
		return
	}

	if other, err := cart.OtherVendorInCart(h.DB, userID, itemID); err != nil {
		fmt.Println("Vendor check error:", err)
		errors.Render(w, "Could not add item to cart", http.StatusInternalServerError)
		return
	} else if other != "" {
		msg := "Your cart has dishes from " + other + ". Check out or clear your cart before ordering from another vendor."
		http.Redirect(w, r, "/menu?message="+url.QueryEscape(msg), http.StatusSeeOther)
		return
	}

	optionIDs := parseOptionIDs(r.Form["option_id"])
	added, err := cart.AddItemWithOptions(h.DB, userID, itemID, optionIDs)
	if err != nil {
		fmt.Println("Add to cart error:", err)
		if strings.Contains(err.Error(), "customization") {
			errors.Render(w, err.Error(), http.StatusBadRequest)
		} else {
			errors.Render(w, "Could not add item to cart", http.StatusInternalServerError)
		}
		return
	}

	if !added {
		http.Redirect(w, r, "/menu?message=Item%20already%20in%20cart", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/menu?message=Successfully%20added%20to%20cart", http.StatusSeeOther)
}

func (h *Handlers) Cart(w http.ResponseWriter, r *http.Request) {
	tmpl := render.MustPage("cart.html", template.FuncMap{
		"naira": naira,
		"multiply": func(price, quantity int) int {
			return price * quantity
		},
	})

	cookie, err := r.Cookie("session_token")
	if err != nil {
		errors.Render(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		errors.Render(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	items, err := cart.GetItems(h.DB, userID)
	if err != nil {
		fmt.Println("Cart load error:", err)
		errors.Render(w, "Could not load cart", http.StatusInternalServerError)
		return
	}

	var total int
	for _, item := range items {
		total += item.PriceKobo * item.Quantity
	}

	message := r.URL.Query().Get("message")

	data := struct {
		Items   []cart.CartItem
		Total   int
		Message string
	}{
		Items:   items,
		Total:   total,
		Message: message,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Cart template error:", err)
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err != nil {
		errors.Render(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		errors.Render(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	itemID, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		errors.Render(w, "Invalid item ID", http.StatusBadRequest)
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
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err != nil {
		errors.Render(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		errors.Render(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	itemID, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		errors.Render(w, "Invalid item ID", http.StatusBadRequest)
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
			errors.Render(w, "Could not update cart", http.StatusInternalServerError)
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
		errors.Render(w, "Invalid action", http.StatusBadRequest)
		return
	}

	if err != nil {
		fmt.Println("Update cart quantity error:", err)
		errors.Render(w, "Could not update cart", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cart", http.StatusSeeOther)
}
