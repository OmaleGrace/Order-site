package handlers

import (
	"Order-site/errors"
	"Order-site/render"
	"html/template"
	"log"
	"net/http"
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
			u.name,
			u.email,
			o.total_kobo,
			COALESCE((SELECT SUM(oi.quantity * oi.price_kobo) FROM order_items oi WHERE oi.order_id = o.id), 0) AS food_subtotal_kobo,
			COALESCE(o.coupon_discount_kobo, 0) AS coupon_discount_kobo,
			COALESCE(o.coupon_code, '') AS coupon_code,
			COALESCE(o.loyalty_points_redeemed, 0) * 100 AS loyalty_discount_kobo,
			o.commission_kobo,
			GREATEST(COALESCE((SELECT SUM(oi.quantity * oi.price_kobo) FROM order_items oi WHERE oi.order_id = o.id), 0) - COALESCE(o.coupon_discount_kobo, 0) - COALESCE(o.loyalty_points_redeemed, 0) * 100 - o.commission_kobo, 0) AS vendor_food_earnings_kobo,
			COALESCE(o.rider_id, 0), COALESCE(r.name, ''), COALESCE(r.phone, ''), COALESCE(o.delivery_status, 'unassigned'),
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
