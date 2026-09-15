package handlers

import (
	"html/template"
	"net/http"
	"Order-site/errors"
)

type AdminOrderItem struct {
	Name      string
	Quantity  int
	PriceKobo int
}

type AdminOrder struct {
	ID        int
	Name      string
	Email     string
	TotalKobo int
	Status    string
	CreatedAt string
	Items     []AdminOrderItem
}

func (h *Handlers) AdminOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := h.DB.Query(`
		SELECT
			o.id,
			u.name,
			u.email,
			o.total_kobo,
			o.status,
			o.created_at
		FROM orders o
		JOIN users u ON u.id = o.user_id
		ORDER BY o.created_at DESC
	`)
	if err != nil {
		errors.Render(w, "Could not load orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orders []AdminOrder

	for rows.Next() {
		var order AdminOrder

		err := rows.Scan(
			&order.ID,
			&order.Name,
			&order.Email,
			&order.TotalKobo,
			&order.Status,
			&order.CreatedAt,
		)

		if err != nil {
			errors.Render(w, "Could not read orders", http.StatusInternalServerError)
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
			errors.Render(w, "Could not load order items", http.StatusInternalServerError)
			return
		}

		for itemRows.Next() {
			var item AdminOrderItem

			err := itemRows.Scan(
				&item.Name,
				&item.Quantity,
				&item.PriceKobo,
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
		template.New("admin-orders.html").
			Funcs(template.FuncMap{
				"naira": func(kobo int) int {
					return kobo / 100
				},
			}).
			ParseFiles("templates/admin-orders.html"),
	)

	err = tmpl.Execute(w, orders)
	if err != nil {
		errors.Render(w, "Could not display orders", http.StatusInternalServerError)
		return
	}
}
