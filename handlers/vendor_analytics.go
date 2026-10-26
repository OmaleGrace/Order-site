package handlers

import (
	"html/template"
	"log"
	"net/http"

	"Order-site/brand"
	"Order-site/errors"
)

type VendorAnalyticsDay struct {
	Label     string
	SalesKobo int
	Orders    int
}

type VendorAnalyticsItem struct {
	Name      string
	Quantity  int
	SalesKobo int
}

type VendorAnalyticsPage struct {
	VendorName       string
	Period           string
	PeriodLabel      string
	SalesKobo        int
	Orders           int
	AverageOrderKobo int
	CommissionKobo   int
	BalanceKobo      int
	DeliveryOrders   int
	PickupOrders     int
	CompletedOrders  int
	CancelledOrders  int
	SalesChange      int
	Days             []VendorAnalyticsDay
	TopItems         []VendorAnalyticsItem
}

func (h *Handlers) VendorAnalytics(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	period := r.URL.Query().Get("period")
	if period != "week" && period != "month" && period != "today" {
		period = "week"
	}
	var interval, previousInterval, label string
	switch period {
	case "today":
		interval, previousInterval, label = "1 day", "1 day", "Today"
	case "month":
		interval, previousInterval, label = "1 month", "1 month", "This month"
	default:
		interval, previousInterval, label = "7 days", "7 days", "This week"
	}

	page := VendorAnalyticsPage{VendorName: vendorName, Period: period, PeriodLabel: label}
	err := h.DB.QueryRow(`
        SELECT COUNT(*), COALESCE(SUM(total_kobo),0), COALESCE(SUM(commission_kobo),0)
        FROM orders
        WHERE vendor_id=$1 AND status NOT IN ('pending','cancelled')
          AND created_at >= CURRENT_TIMESTAMP - $2::interval
    `, vendorID, interval).Scan(&page.Orders, &page.SalesKobo, &page.CommissionKobo)
	if err != nil {
		log.Printf("vendor analytics summary: %v", err)
		errors.Render(w, "Could not load analytics", http.StatusInternalServerError)
		return
	}
	if page.Orders > 0 {
		page.AverageOrderKobo = page.SalesKobo / page.Orders
	}

	var paidOut int
	if err := h.DB.QueryRow(`SELECT COALESCE(SUM(amount_kobo),0) FROM vendor_payouts WHERE vendor_id=$1`, vendorID).Scan(&paidOut); err != nil {
		log.Printf("vendor analytics payout: %v", err)
		errors.Render(w, "Could not load analytics", http.StatusInternalServerError)
		return
	}
	page.BalanceKobo = page.SalesKobo - page.CommissionKobo - paidOut

	if err := h.DB.QueryRow(`
        SELECT COUNT(*) FILTER (WHERE delivery_method='delivery'), COUNT(*) FILTER (WHERE delivery_method='pickup'),
               COUNT(*) FILTER (WHERE status='completed'), COUNT(*) FILTER (WHERE status='cancelled')
        FROM orders WHERE vendor_id=$1 AND created_at >= CURRENT_TIMESTAMP - $2::interval
    `, vendorID, interval).Scan(&page.DeliveryOrders, &page.PickupOrders, &page.CompletedOrders, &page.CancelledOrders); err != nil {
		log.Printf("vendor analytics fulfillment: %v", err)
		errors.Render(w, "Could not load analytics", http.StatusInternalServerError)
		return
	}

	var current, previous int
	if err := h.DB.QueryRow(`SELECT COALESCE(SUM(total_kobo),0) FROM orders WHERE vendor_id=$1 AND status NOT IN ('pending','cancelled') AND created_at >= CURRENT_TIMESTAMP - $2::interval`, vendorID, interval).Scan(&current); err == nil {
		if err := h.DB.QueryRow(`SELECT COALESCE(SUM(total_kobo),0) FROM orders WHERE vendor_id=$1 AND status NOT IN ('pending','cancelled') AND created_at >= CURRENT_TIMESTAMP - ($2::interval * 2) AND created_at < CURRENT_TIMESTAMP - $2::interval`, vendorID, previousInterval).Scan(&previous); err == nil && previous > 0 {
			page.SalesChange = int(float64(current-previous) * 100 / float64(previous))
		}
	}

	rows, err := h.DB.Query(`
        SELECT TO_CHAR(day, 'Dy'), COALESCE(SUM(o.total_kobo),0), COUNT(o.id)
        FROM generate_series(CURRENT_DATE - INTERVAL '6 days', CURRENT_DATE, INTERVAL '1 day') AS day
        LEFT JOIN orders o ON o.vendor_id=$1 AND o.created_at >= day AND o.created_at < day + INTERVAL '1 day' AND o.status NOT IN ('pending','cancelled')
        GROUP BY day ORDER BY day
    `, vendorID)
	if err != nil {
		log.Printf("vendor analytics daily trend: %v", err)
		errors.Render(w, "Could not load analytics", http.StatusInternalServerError)
		return
	}
	for rows.Next() {
		var d VendorAnalyticsDay
		if err := rows.Scan(&d.Label, &d.SalesKobo, &d.Orders); err != nil {
			rows.Close()
			errors.Render(w, "Could not read analytics", http.StatusInternalServerError)
			return
		}
		page.Days = append(page.Days, d)
	}
	rows.Close()

	rows, err = h.DB.Query(`
        SELECT m.name, SUM(oi.quantity), SUM(oi.quantity * oi.price_kobo)
        FROM order_items oi JOIN orders o ON o.id=oi.order_id JOIN menu_items m ON m.id=oi.menu_item_id
        WHERE o.vendor_id=$1 AND o.status NOT IN ('pending','cancelled') AND o.created_at >= CURRENT_TIMESTAMP - $2::interval
        GROUP BY m.id,m.name ORDER BY SUM(oi.quantity) DESC, SUM(oi.quantity*oi.price_kobo) DESC LIMIT 6
    `, vendorID, interval)
	if err != nil {
		log.Printf("vendor analytics top items: %v", err)
		errors.Render(w, "Could not load analytics", http.StatusInternalServerError)
		return
	}
	for rows.Next() {
		var item VendorAnalyticsItem
		if err := rows.Scan(&item.Name, &item.Quantity, &item.SalesKobo); err != nil {
			rows.Close()
			errors.Render(w, "Could not read analytics", http.StatusInternalServerError)
			return
		}
		page.TopItems = append(page.TopItems, item)
	}
	rows.Close()

	funcs := template.FuncMap{
		"money": money,
		"abs": func(n int) int {
			if n < 0 {
				return -n
			}
			return n
		},
		"add": func(a, b int) int { return a + b },
		"percent": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a * 100 / b
		},
	}
	tmpl, err := template.New("vendor-analytics.html").Funcs(funcs).Funcs(brand.Funcs()).ParseFiles("templates/vendor-analytics.html")
	if err != nil {
		log.Printf("vendor analytics template: %v", err)
		errors.Render(w, "Could not display analytics", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor analytics execute: %v", err)
	}
}
