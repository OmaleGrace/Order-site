package handlers

import (
	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type CustomerOrderItem struct {
	MenuItemID int
	Name       string
	Quantity   int
	PriceKobo  int
	ImageURL   string
	Options    []string
}

type CustomerOrder struct {
	ID                  int
	Reference           string
	VendorID            int
	VendorName          string
	VendorSlug          string
	TotalKobo           int
	FoodSubtotalKobo    int
	LoyaltyDiscountKobo int
	DeliveryFeeKobo     int
	Status              string
	CreatedAt           time.Time
	DeliveryMethod      string
	DeliveryAddress     string
	Reviewed            bool
	ScheduledFor        *time.Time
	RiderName           string
	RiderPhone          string
	DeliveryStatus      string
	Items               []CustomerOrderItem
}

func (h *Handlers) MyOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
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

	rows, err := h.DB.Query(`
		SELECT orders.id, orders.vendor_id, v.name, v.slug, orders.total_kobo,
		       COALESCE((SELECT SUM(oi.quantity * oi.price_kobo) FROM order_items oi WHERE oi.order_id = orders.id), 0),
		       COALESCE(orders.loyalty_points_redeemed, 0) * 100, orders.delivery_fee_kobo, orders.status, orders.created_at, orders.delivery_method,
		       TRIM(BOTH ', ' FROM CONCAT_WS(', ', delivery_address, delivery_lga, delivery_state)),
               EXISTS(SELECT 1 FROM order_reviews r WHERE r.order_id = orders.id),
               orders.scheduled_for, COALESCE(r.name,''), COALESCE(r.phone,''), COALESCE(orders.delivery_status,'unassigned')
		FROM orders
		JOIN vendors v ON v.id = orders.vendor_id
        LEFT JOIN riders r ON r.id = orders.rider_id
		WHERE orders.user_id = $1
		ORDER BY orders.created_at DESC
	`, userID)

	if err != nil {
		errors.Render(w, "Could not load orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orders []CustomerOrder

	for rows.Next() {
		var order CustomerOrder

		err := rows.Scan(
			&order.ID,
			&order.VendorID,
			&order.VendorName,
			&order.VendorSlug,
			&order.TotalKobo,
			&order.FoodSubtotalKobo,
			&order.LoyaltyDiscountKobo,
			&order.DeliveryFeeKobo,
			&order.Status,
			&order.CreatedAt,
			&order.DeliveryMethod,
			&order.DeliveryAddress,
			&order.Reviewed,
			&order.ScheduledFor,
			&order.RiderName,
			&order.RiderPhone,
			&order.DeliveryStatus,
		)
		if err != nil {
			errors.Render(w, "Could not read orders", http.StatusInternalServerError)
			return
		}
		order.Reference = publicOrderReference(order.ID, order.CreatedAt)

		itemRows, err := h.DB.Query(`
			SELECT
				oi.menu_item_id,
				m.name,
				oi.quantity,
				oi.price_kobo,
				COALESCE(m.image_url, '')
			FROM order_items oi
			JOIN menu_items m ON m.id = oi.menu_item_id
			WHERE oi.order_id = $1
		`, order.ID)

		if err != nil {
			errors.Render(w, "Could not load order items", http.StatusInternalServerError)
			return
		}

		for itemRows.Next() {
			var item CustomerOrderItem

			err := itemRows.Scan(
				&item.Name,
				&item.Quantity,
				&item.PriceKobo,
				&item.ImageURL,
			)

			if err != nil {
				itemRows.Close()
				errors.Render(w, "Could not read order items", http.StatusInternalServerError)
				return
			}

			optionRows, optErr := h.DB.Query(`SELECT name, price_kobo FROM order_item_options WHERE order_item_id = (SELECT oi2.id FROM order_items oi2 WHERE oi2.order_id=$1 AND oi2.menu_item_id=$2 ORDER BY oi2.id DESC LIMIT 1) ORDER BY id`, order.ID, item.MenuItemID)
			if optErr == nil {
				for optionRows.Next() {
					var n string
					var p int
					if optionRows.Scan(&n, &p) == nil {
						if p > 0 {
							item.Options = append(item.Options, fmt.Sprintf("%s (+₦%d)", n, p/100))
						} else {
							item.Options = append(item.Options, n)
						}
					}
				}
				optionRows.Close()
			}
			order.Items = append(order.Items, item)
		}

		itemRows.Close()

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read orders", http.StatusInternalServerError)
		return
	}

	data := struct {
		Orders  []CustomerOrder
		Message string
	}{Orders: orders, Message: r.URL.Query().Get("message")}

	tmpl := template.Must(
		template.New("orders.html").
			Funcs(template.FuncMap{
				"naira": func(kobo int) int { return kobo / 100 },
				"date":  func(value time.Time) string { return value.Format("02 Jan 2006, 3:04 PM") },
			}).
			Funcs(brand.Funcs()).
			ParseFiles("templates/orders.html"),
	)

	err = tmpl.Execute(w, data)
	if err != nil {
		errors.Render(w, "Could not display orders", http.StatusInternalServerError)
		return
	}
}

// Reorder adds the items from a previous eligible order back into the customer's cart.
// It keeps any existing items from the same vendor and increases quantities for duplicates.
func (h *Handlers) Reorder(w http.ResponseWriter, r *http.Request) {
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

	orderID, err := strconv.Atoi(r.FormValue("order_id"))
	if err != nil || orderID <= 0 {
		errors.Render(w, "Invalid order", http.StatusBadRequest)
		return
	}

	type reorderItem struct {
		MenuItemID int
		Quantity   int
		PriceKobo  int
		VendorID   int
	}

	rows, err := h.DB.Query(`
		SELECT oi.menu_item_id, oi.quantity, m.price_kobo, m.vendor_id
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		JOIN menu_items m ON m.id = oi.menu_item_id
		JOIN vendors v ON v.id = m.vendor_id
		WHERE oi.order_id = $1
		  AND o.user_id = $2
		  AND o.status NOT IN ('pending', 'cancelled')
		  AND v.status = 'active'
		  AND v.paid_until > NOW()
		ORDER BY oi.id
	`, orderID, userID)
	if err != nil {
		log.Printf("reorder: query failed: %v", err)
		errors.Render(w, "Could not prepare your order again", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []reorderItem
	vendorID := 0
	for rows.Next() {
		var item reorderItem
		if err := rows.Scan(&item.MenuItemID, &item.Quantity, &item.PriceKobo, &item.VendorID); err != nil {
			errors.Render(w, "Could not read the previous order", http.StatusInternalServerError)
			return
		}
		if item.Quantity < 1 {
			continue
		}
		if vendorID == 0 {
			vendorID = item.VendorID
		} else if vendorID != item.VendorID {
			log.Printf("reorder: order %d contains multiple vendors", orderID)
			errors.Render(w, "This order cannot be reordered as one cart", http.StatusBadRequest)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read the previous order", http.StatusInternalServerError)
		return
	}
	if len(items) == 0 {
		http.Redirect(w, r, "/orders?message="+url.QueryEscape("Those items are no longer available from the shop."), http.StatusSeeOther)
		return
	}

	var otherVendor string
	err = h.DB.QueryRow(`
		SELECT v.name
		FROM cart_items ci
		JOIN menu_items m ON m.id = ci.menu_item_id
		JOIN vendors v ON v.id = m.vendor_id
		WHERE ci.user_id = $1 AND m.vendor_id <> $2
		LIMIT 1
	`, userID, vendorID).Scan(&otherVendor)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("reorder: cart vendor check failed: %v", err)
		errors.Render(w, "Could not check your cart", http.StatusInternalServerError)
		return
	}
	if otherVendor != "" {
		msg := fmt.Sprintf("Your cart has dishes from %s. Check out or clear your cart before reordering.", otherVendor)
		http.Redirect(w, r, "/orders?message="+url.QueryEscape(msg), http.StatusSeeOther)
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		errors.Render(w, "Could not prepare your cart", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	added := 0
	for _, item := range items {
		var existing int
		err := tx.QueryRow(`
			SELECT quantity FROM cart_items
			WHERE user_id = $1 AND menu_item_id = $2
			FOR UPDATE
		`, userID, item.MenuItemID).Scan(&existing)

		switch err {
		case nil:
			if _, err = tx.Exec(`
				UPDATE cart_items SET quantity = quantity + $1
				WHERE user_id = $2 AND menu_item_id = $3
			`, item.Quantity, userID, item.MenuItemID); err != nil {
				log.Printf("reorder: quantity update failed: %v", err)
				errors.Render(w, "Could not update your cart", http.StatusInternalServerError)
				return
			}
			added += item.Quantity
		case sql.ErrNoRows:
			if _, err = tx.Exec(`
				INSERT INTO cart_items (user_id, menu_item_id, quantity, price_kobo_at_addition)
				VALUES ($1, $2, $3, $4)
			`, userID, item.MenuItemID, item.Quantity, item.PriceKobo); err != nil {
				log.Printf("reorder: insert failed: %v", err)
				errors.Render(w, "Could not add the previous order to your cart", http.StatusInternalServerError)
				return
			}
			added += item.Quantity
		default:
			log.Printf("reorder: cart lookup failed: %v", err)
			errors.Render(w, "Could not update your cart", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("reorder: commit failed: %v", err)
		errors.Render(w, "Could not save your reordered items", http.StatusInternalServerError)
		return
	}

	message := fmt.Sprintf("Added %d item%s from your previous order to the cart.", added, func() string {
		if added == 1 {
			return ""
		}
		return "s"
	}())
	http.Redirect(w, r, "/cart?message="+url.QueryEscape(message), http.StatusSeeOther)
}
