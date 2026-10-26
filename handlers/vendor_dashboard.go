package handlers

import (
	"html/template"
	"log"
	"net/http"

	"Order-site/brand"
	"Order-site/errors"
)

type VendorDashboardOrder struct {
	ID        int
	Customer  string
	TotalKobo int
	Status    string
	CreatedAt string
	Method    string
}

type VendorDashboardPage struct {
	VendorName     string
	ShopStatus     string
	Verified       bool
	PayoutVerified bool
	TodayOrders    int
	TodaySalesKobo int
	ActiveOrders   int
	TotalOrders    int
	Earnings       VendorEarnings
	RecentOrders   []VendorDashboardOrder
	Notice         string
}

func (h *Handlers) VendorDashboard(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	var page VendorDashboardPage
	page.VendorName = vendorName
	page.Notice = r.URL.Query().Get("msg")

	if err := h.DB.QueryRow(`
        SELECT status, COALESCE(verified, FALSE), COALESCE(payout_account_verified, FALSE)
        FROM vendors WHERE id = $1
    `, vendorID).Scan(&page.ShopStatus, &page.Verified, &page.PayoutVerified); err != nil {
		log.Printf("vendor dashboard: vendor query failed: %v", err)
		errors.Render(w, "Could not load your dashboard", http.StatusInternalServerError)
		return
	}

	if err := h.DB.QueryRow(`
        SELECT COUNT(*), COALESCE(SUM(total_kobo), 0)
        FROM orders
        WHERE vendor_id = $1
          AND status NOT IN ('pending', 'cancelled')
          AND created_at >= CURRENT_DATE
          AND created_at < CURRENT_DATE + INTERVAL '1 day'
    `, vendorID).Scan(&page.TodayOrders, &page.TodaySalesKobo); err != nil {
		log.Printf("vendor dashboard: today stats failed: %v", err)
		errors.Render(w, "Could not load dashboard statistics", http.StatusInternalServerError)
		return
	}

	if err := h.DB.QueryRow(`
        SELECT COUNT(*)
        FROM orders
        WHERE vendor_id = $1
          AND status IN ('preparing', 'ready')
    `, vendorID).Scan(&page.ActiveOrders); err != nil {
		log.Printf("vendor dashboard: active orders failed: %v", err)
		errors.Render(w, "Could not load dashboard statistics", http.StatusInternalServerError)
		return
	}

	if err := h.DB.QueryRow(`
        SELECT COUNT(*)
        FROM orders
        WHERE vendor_id = $1 AND status NOT IN ('pending', 'cancelled')
    `, vendorID).Scan(&page.TotalOrders); err != nil {
		log.Printf("vendor dashboard: total orders failed: %v", err)
		errors.Render(w, "Could not load dashboard statistics", http.StatusInternalServerError)
		return
	}

	var earningsErr error
	page.Earnings, earningsErr = h.vendorEarnings(vendorID)
	if earningsErr != nil {
		log.Printf("vendor dashboard: earnings failed: %v", earningsErr)
		errors.Render(w, "Could not load earnings", http.StatusInternalServerError)
		return
	}

	rows, err := h.DB.Query(`
        SELECT o.id, u.name, o.total_kobo, o.status,
               TO_CHAR(o.created_at, 'DD Mon, HH24:MI'), o.delivery_method
        FROM orders o
        JOIN users u ON u.id = o.user_id
        WHERE o.vendor_id = $1 AND o.status <> 'pending'
        ORDER BY o.created_at DESC
        LIMIT 6
    `, vendorID)
	if err != nil {
		log.Printf("vendor dashboard: recent orders query failed: %v", err)
		errors.Render(w, "Could not load recent orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var order VendorDashboardOrder
		if err := rows.Scan(&order.ID, &order.Customer, &order.TotalKobo, &order.Status, &order.CreatedAt, &order.Method); err != nil {
			log.Printf("vendor dashboard: recent order scan failed: %v", err)
			errors.Render(w, "Could not read recent orders", http.StatusInternalServerError)
			return
		}
		page.RecentOrders = append(page.RecentOrders, order)
	}
	if err := rows.Err(); err != nil {
		log.Printf("vendor dashboard: recent orders rows failed: %v", err)
		errors.Render(w, "Could not read recent orders", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("vendor-dashboard.html").
		Funcs(template.FuncMap{"money": money, "naira": naira}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/vendor-dashboard.html")
	if err != nil {
		log.Printf("vendor dashboard: template failed: %v", err)
		errors.Render(w, "Could not display dashboard", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor dashboard: execute failed: %v", err)
	}
}
