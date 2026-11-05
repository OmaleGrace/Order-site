package handlers

import (
	"Order-site/errors"
	"Order-site/render"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
)

type AdminOrderItem struct {
	Name      string
	Quantity  int
	PriceKobo int
}

type AdminOrder struct {
	ID                     int
	Name                   string
	Email                  string
	TotalKobo              int
	FoodSubtotalKobo       int
	CouponDiscountKobo     int
	CouponCode             string
	LoyaltyDiscountKobo    int
	CommissionKobo         int
	VendorFoodEarningsKobo int
	RiderID                int
	RiderName              string
	RiderPhone             string
	RiderMapURL            string
	RiderMapEmbedURL       string
	RiderLocationAvailable bool
	RiderLocationUpdated   string
	DeliveryStatus         string
	VendorName             string
	Status                 string
	CreatedAt              string
	DeliveryMethod         string
	DeliveryAddress        string
	DeliveryPhone          string
	DeliveryFeeKobo        int
	Items                  []AdminOrderItem
}

func (h *Handlers) AdminOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := h.DB.Query(`
		SELECT
			o.id,
			COALESCE(NULLIF(o.guest_name, ''), u.name),
			COALESCE(NULLIF(o.guest_email, ''), u.email),
			o.total_kobo,
			COALESCE((SELECT SUM(oi.quantity * oi.price_kobo) FROM order_items oi WHERE oi.order_id = o.id), 0) AS food_subtotal_kobo,
			COALESCE(o.coupon_discount_kobo, 0) AS coupon_discount_kobo,
			COALESCE(o.coupon_code, '') AS coupon_code,
			COALESCE(o.loyalty_points_redeemed, 0) * 100 AS loyalty_discount_kobo,
			o.commission_kobo,
			GREATEST(COALESCE((SELECT SUM(oi.quantity * oi.price_kobo) FROM order_items oi WHERE oi.order_id = o.id), 0) - COALESCE(o.coupon_discount_kobo, 0) - COALESCE(o.loyalty_points_redeemed, 0) * 100 - o.commission_kobo, 0) AS vendor_food_earnings_kobo,
			COALESCE(o.rider_id, 0), COALESCE(r.name, ''), COALESCE(r.phone, ''),
            CASE WHEN COALESCE(o.rider_sharing_location,FALSE) THEN COALESCE(o.rider_latitude,0) ELSE 0 END,
            CASE WHEN COALESCE(o.rider_sharing_location,FALSE) THEN COALESCE(o.rider_longitude,0) ELSE 0 END,
            COALESCE(o.rider_sharing_location,FALSE), COALESCE(TO_CHAR(o.rider_location_updated_at,'DD Mon YYYY, HH24:MI'),''), COALESCE(o.delivery_status, 'unassigned'),
			COALESCE(v.name, '') AS vendor_name,
			o.status,
			o.created_at,
			o.delivery_method, o.delivery_address || ', ' || o.delivery_lga || ', ' || o.delivery_state, o.delivery_phone, o.delivery_fee_kobo
		FROM orders o
		JOIN users u ON u.id = o.user_id
		LEFT JOIN vendors v ON v.id = o.vendor_id
		LEFT JOIN riders r ON r.id = o.rider_id
		ORDER BY o.created_at DESC
	`)
	if err != nil {
		log.Printf("admin orders: query failed: %v", err)
		errors.Render(w, "Could not load orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orders []AdminOrder
	var riders []AdminRider
	riderRows, riderErr := h.DB.Query(`SELECT id,name,phone,vehicle,active,0,0 FROM riders WHERE active=TRUE ORDER BY name`)
	if riderErr == nil {
		defer riderRows.Close()
		for riderRows.Next() {
			var rr AdminRider
			if e := riderRows.Scan(&rr.ID, &rr.Name, &rr.Phone, &rr.Vehicle, &rr.Active, &rr.Assigned, &rr.Completed); e == nil {
				riders = append(riders, rr)
			}
		}
	}

	for rows.Next() {
		var order AdminOrder
		var riderLat, riderLon float64

		err := rows.Scan(
			&order.ID,
			&order.Name,
			&order.Email,
			&order.TotalKobo,
			&order.FoodSubtotalKobo,
			&order.CouponDiscountKobo,
			&order.CouponCode,
			&order.LoyaltyDiscountKobo,
			&order.CommissionKobo,
			&order.VendorFoodEarningsKobo,
			&order.RiderID,
			&order.RiderName,
			&order.RiderPhone,
			&riderLat, &riderLon, &order.RiderLocationAvailable, &order.RiderLocationUpdated,
			&order.DeliveryStatus,
			&order.VendorName,
			&order.Status,
			&order.CreatedAt,
			&order.DeliveryMethod,
			&order.DeliveryAddress,
			&order.DeliveryPhone,
			&order.DeliveryFeeKobo,
		)

		if err != nil {
			errors.Render(w, "Could not read orders", http.StatusInternalServerError)
			return
		}
		if order.RiderLocationAvailable && (riderLat != 0 || riderLon != 0) {
			order.RiderMapURL = fmt.Sprintf("https://www.openstreetmap.org/?mlat=%.6f&mlon=%.6f#map=16/%.6f/%.6f", riderLat, riderLon, riderLat, riderLon)
			order.RiderMapEmbedURL = fmt.Sprintf("https://www.openstreetmap.org/export/embed.html?bbox=%.6f%%2C%.6f%%2C%.6f%%2C%.6f&layer=mapnik&marker=%.6f%%2C%.6f", riderLon-.01, riderLat-.01, riderLon+.01, riderLat+.01, riderLat, riderLon)
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

	tmpl := render.MustPage("admin-orders.html", template.FuncMap{
		"naira": func(kobo int) int {
			return kobo / 100
		},
		"sub": func(a, b int) int { return a - b },
	})

	err = tmpl.Execute(w, struct {
		Orders []AdminOrder
		Riders []AdminRider
	}{Orders: orders, Riders: riders})
	if err != nil {
		errors.Render(w, "Could not display orders", http.StatusInternalServerError)
		return
	}
}

// AdminRiderLocation returns the latest active rider location for one order.
// The route is protected by the admin middleware in main.go.
func (h *Handlers) AdminRiderLocation(w http.ResponseWriter, r *http.Request) {
	orderID, err := strconv.Atoi(r.PathValue("orderID"))
	if err != nil || orderID < 1 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var lat, lon, accuracy float64
	var available bool
	var updated string
	err = h.DB.QueryRow(`SELECT COALESCE(rider_latitude,0), COALESCE(rider_longitude,0), COALESCE(rider_location_accuracy,0), COALESCE(rider_sharing_location,FALSE), COALESCE(TO_CHAR(rider_location_updated_at,'DD Mon YYYY, HH24:MI:SS'),'') FROM orders WHERE id=$1 AND delivery_method='delivery'`, orderID).Scan(&lat, &lon, &accuracy, &available, &updated)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !available || (lat == 0 && lon == 0) {
		available = false
	}
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"available": available, "latitude": lat, "longitude": lon, "accuracy": accuracy, "updated_at": updated,
	})
}
