package handlers

import (
	"html/template"
	"net/http"
)

type AdminOrder struct {
	ID        int
	Name      string
	Email     string
	TotalKobo int
	Status    string
	CreatedAt string
}

func (h *Handlers) AdminOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		http.Error(w, "Could not load orders", http.StatusInternalServerError)
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
			http.Error(w, "Could not read orders", http.StatusInternalServerError)
			return
		}

		orders = append(orders, order)
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
		http.Error(w, "Could not display orders", http.StatusInternalServerError)
		return
	}
}