package handlers

import (
	"html/template"
	"log"
	"net/http"

	"Order-site/brand"
	"Order-site/errors"
)

type VendorOrderItem struct {
	Name      string
	Quantity  int
	PriceKobo int
}

type VendorOrder struct {
	ID              int
	Name            string
	Email           string
	Phone           string
	TotalKobo       int
	Status          string
	CreatedAt       string
	DeliveryMethod  string
	DeliveryAddress string
	DeliveryPhone   string
	DeliveryFeeKobo int
	Items           []VendorOrderItem
}

type VendorOrdersPage struct {
	VendorName string
	Orders     []VendorOrder
}

func (h *Handlers) VendorOrders(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	rows, err := h.DB.Query(`
		SELECT o.id, u.name, u.email, COALESCE(u.phone, ''), o.total_kobo, o.status,
		       TO_CHAR(o.created_at, 'DD Mon YYYY, HH24:MI'),
		       o.delivery_method, o.delivery_address || ', ' || o.delivery_lga || ', ' || o.delivery_state, o.delivery_phone, o.delivery_fee_kobo
		FROM orders o
		JOIN users u ON u.id = o.user_id
		WHERE o.vendor_id = $1 AND o.status <> 'pending'
		ORDER BY o.created_at DESC
	`, vendorID)
	if err != nil {
		log.Printf("vendor orders: query failed: %v", err)
		errors.Render(w, "Could not load orders", http.StatusInternalServerError)
		return
	}

	var orders []VendorOrder
	for rows.Next() {
		var o VendorOrder
		if err := rows.Scan(&o.ID, &o.Name, &o.Email, &o.Phone, &o.TotalKobo, &o.Status, &o.CreatedAt, &o.DeliveryMethod, &o.DeliveryAddress, &o.DeliveryPhone, &o.DeliveryFeeKobo); err != nil {
			rows.Close()
			log.Printf("vendor orders: scan failed: %v", err)
			errors.Render(w, "Could not read orders", http.StatusInternalServerError)
			return
		}
		orders = append(orders, o)
	}
	rows.Close()

	for i := range orders {
		itemRows, err := h.DB.Query(`
			SELECT m.name, oi.quantity, oi.price_kobo
			FROM order_items oi
			JOIN menu_items m ON m.id = oi.menu_item_id
			WHERE oi.order_id = $1
		`, orders[i].ID)
		if err != nil {
			log.Printf("vendor orders: items query failed: %v", err)
			errors.Render(w, "Could not load order items", http.StatusInternalServerError)
			return
		}
		for itemRows.Next() {
			var it VendorOrderItem
			if err := itemRows.Scan(&it.Name, &it.Quantity, &it.PriceKobo); err != nil {
				itemRows.Close()
				log.Printf("vendor orders: items scan failed: %v", err)
				errors.Render(w, "Could not read order items", http.StatusInternalServerError)
				return
			}
			orders[i].Items = append(orders[i].Items, it)
		}
		itemRows.Close()
	}

	tmpl, err := template.New("vendor-orders.html").
		Funcs(template.FuncMap{"naira": naira}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/vendor-orders.html")
	if err != nil {
		log.Printf("vendor orders: template failed: %v", err)
		errors.Render(w, "Could not display orders", http.StatusInternalServerError)
		return
	}

	page := VendorOrdersPage{VendorName: vendorName, Orders: orders}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor orders: execute failed: %v", err)
	}
}
