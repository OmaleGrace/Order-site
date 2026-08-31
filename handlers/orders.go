package handlers

import (
	"html/template"
	"net/http"
	"strconv"
)

type CustomerOrderItem struct {
	Name      string
	Quantity  int
	PriceKobo int
}

type CustomerOrder struct {
	ID        int
	TotalKobo int
	Status    string
	CreatedAt string
	Items     []CustomerOrderItem
}

func (h *Handlers) MyOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
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

	rows, err := h.DB.Query(`
		SELECT id, total_kobo, status, created_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)

	if err != nil {
		http.Error(w, "Could not load orders", http.StatusInternalServerError)
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
			http.Error(w, "Could not read orders", http.StatusInternalServerError)
			return
		}

		itemRows, err := h.DB.Query(`
			SELECT
				m.name,
				oi.quantity,
				oi.price_kobo
			FROM order_items oi
			JOIN menu_items m ON m.id = oi.menu_item_id
			WHERE oi.order_id = $1
		`, order.ID)

		if err != nil {
			http.Error(w, "Could not load order items", http.StatusInternalServerError)
			return
		}

		for itemRows.Next() {
			var item CustomerOrderItem

			err := itemRows.Scan(
				&item.Name,
				&item.Quantity,
				&item.PriceKobo,
			)

			if err != nil {
				itemRows.Close()
				http.Error(w, "Could not read order items", http.StatusInternalServerError)
				return
			}

			order.Items = append(order.Items, item)
		}

		itemRows.Close()

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Could not read orders", http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(
		template.New("orders.html").
			Funcs(template.FuncMap{
				"naira": func(kobo int) int {
					return kobo / 100
				},
			}).
			ParseFiles("templates/orders.html"),
	)

	err = tmpl.Execute(w, orders)
	if err != nil {
		http.Error(w, "Could not display orders", http.StatusInternalServerError)
		return
	}
}
