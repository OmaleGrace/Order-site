package handlers

import (
	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
	"html/template"
	"net/http"
	"time"
)

type CustomerOrderItem struct {
	Name      string
	Quantity  int
	PriceKobo int
	ImageURL  string
}

type CustomerOrder struct {
	ID        int
	Reference string
	TotalKobo int
	Status    string
	CreatedAt time.Time
	Items     []CustomerOrderItem
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
		SELECT id, total_kobo, status, created_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
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
			&order.TotalKobo,
			&order.Status,
			&order.CreatedAt,
		)
		if err != nil {
			errors.Render(w, "Could not read orders", http.StatusInternalServerError)
			return
		}
		order.Reference = publicOrderReference(order.ID, order.CreatedAt)

		itemRows, err := h.DB.Query(`
			SELECT
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

			order.Items = append(order.Items, item)
		}

		itemRows.Close()

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read orders", http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(
		template.New("orders.html").
			Funcs(template.FuncMap{
				"naira": func(kobo int) int { return kobo / 100 },
				"date":  func(value time.Time) string { return value.Format("02 Jan 2006, 3:04 PM") },
			}).
			Funcs(brand.Funcs()).
			ParseFiles("templates/orders.html"),
	)

	err = tmpl.Execute(w, orders)
	if err != nil {
		errors.Render(w, "Could not display orders", http.StatusInternalServerError)
		return
	}
}
