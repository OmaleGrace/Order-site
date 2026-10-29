package handlers

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"Order-site/errors"
	"Order-site/render"
)

type VendorOrderItem struct {
	Name      string
	Quantity  int
	PriceKobo int
}

type VendorOrder struct {
	ID                     int
	Name                   string
	Email                  string
	Phone                  string
	TotalKobo              int
	FoodSubtotalKobo       int
	LoyaltyDiscountKobo    int
	CouponDiscountKobo     int
	CouponCode             string
	CommissionKobo         int
	VendorFoodEarningsKobo int
	Status                 string
	CreatedAt              string
	DeliveryMethod         string
	DeliveryAddress        string
	DeliveryPhone          string
	DeliveryFeeKobo        int
	RiderName              string
	RiderPhone             string
	DeliveryStatus         string
	ScheduledFor           *time.Time
	Items                  []VendorOrderItem
}

type VendorOrdersPage struct {
	VendorName      string
	ScheduledOrders []VendorOrder
	Orders          []VendorOrder
}

func (h *Handlers) VendorOrders(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	queryOrders := func(statusFilter, orderBy string) ([]VendorOrder, error) {
		rows, err := h.DB.Query(`
			SELECT o.id, u.name, u.email, COALESCE(u.phone, ''), o.total_kobo,
			       COALESCE((SELECT SUM(oi.quantity * oi.price_kobo) FROM order_items oi WHERE oi.order_id = o.id), 0) AS food_subtotal_kobo,
			   COALESCE(o.loyalty_points_redeemed, 0) * 100 AS loyalty_discount_kobo,
			   COALESCE(o.coupon_discount_kobo, 0) AS coupon_discount_kobo,
			   COALESCE(o.coupon_code, '') AS coupon_code,
			   o.commission_kobo,
			   GREATEST(COALESCE((SELECT SUM(oi.quantity * oi.price_kobo) FROM order_items oi WHERE oi.order_id = o.id), 0) - COALESCE(o.loyalty_points_redeemed, 0) * 100 - COALESCE(o.coupon_discount_kobo, 0) - o.commission_kobo, 0) AS vendor_food_earnings_kobo,
			       o.status,
			       TO_CHAR(o.created_at, 'DD Mon YYYY, HH24:MI'),
			       o.delivery_method, o.delivery_address || ', ' || o.delivery_lga || ', ' || o.delivery_state, o.delivery_phone, o.delivery_fee_kobo, COALESCE(r.name, ''), COALESCE(r.phone, ''), COALESCE(o.delivery_status, 'unassigned'), o.scheduled_for
			FROM orders o
			JOIN users u ON u.id = o.user_id
            LEFT JOIN riders r ON r.id = o.rider_id
			WHERE o.vendor_id = $1 AND `+statusFilter+`
			ORDER BY `+orderBy+`
		`, vendorID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var result []VendorOrder
		for rows.Next() {
			var o VendorOrder
			if err := rows.Scan(&o.ID, &o.Name, &o.Email, &o.Phone, &o.TotalKobo, &o.FoodSubtotalKobo, &o.LoyaltyDiscountKobo, &o.CouponDiscountKobo, &o.CouponCode, &o.CommissionKobo, &o.VendorFoodEarningsKobo, &o.Status, &o.CreatedAt, &o.DeliveryMethod, &o.DeliveryAddress, &o.DeliveryPhone, &o.DeliveryFeeKobo, &o.RiderName, &o.RiderPhone, &o.DeliveryStatus, &o.ScheduledFor); err != nil {
				return nil, err
			}
			result = append(result, o)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return result, nil
	}

	scheduledOrders, err := queryOrders("o.status = 'scheduled' AND o.scheduled_for IS NOT NULL", "o.scheduled_for ASC")
	if err != nil {
		log.Printf("vendor orders: scheduled query failed: %v", err)
		errors.Render(w, "Could not load scheduled orders", http.StatusInternalServerError)
		return
	}

	orders, err := queryOrders("o.status NOT IN ('pending', 'scheduled')", "o.created_at DESC")
	if err != nil {
		log.Printf("vendor orders: query failed: %v", err)
		errors.Render(w, "Could not load orders", http.StatusInternalServerError)
		return
	}

	loadItems := func(orderList []VendorOrder) error {
		for i := range orderList {
			itemRows, err := h.DB.Query(`
				SELECT m.name, oi.quantity, oi.price_kobo
				FROM order_items oi
				JOIN menu_items m ON m.id = oi.menu_item_id
				WHERE oi.order_id = $1
			`, orderList[i].ID)
			if err != nil {
				return err
			}
			for itemRows.Next() {
				var it VendorOrderItem
				if err := itemRows.Scan(&it.Name, &it.Quantity, &it.PriceKobo); err != nil {
					itemRows.Close()
					return err
				}
				orderList[i].Items = append(orderList[i].Items, it)
			}
			if err := itemRows.Err(); err != nil {
				itemRows.Close()
				return err
			}
			itemRows.Close()
		}
		return nil
	}
	if err := loadItems(scheduledOrders); err != nil {
		log.Printf("vendor orders: scheduled items query failed: %v", err)
		errors.Render(w, "Could not load scheduled order items", http.StatusInternalServerError)
		return
	}
	if err := loadItems(orders); err != nil {
		log.Printf("vendor orders: items query failed: %v", err)
		errors.Render(w, "Could not load order items", http.StatusInternalServerError)
		return
	}

	tmpl, err := render.Page("vendor-orders.html", template.FuncMap{"naira": naira, "sub": func(a, b int) int { return a - b }, "commissionPercent": commissionPercent, "dateTime": func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.Format("02 Jan 2006, 3:04 PM")
	}})
	if err != nil {
		log.Printf("vendor orders: template failed: %v", err)
		errors.Render(w, "Could not display orders", http.StatusInternalServerError)
		return
	}

	page := VendorOrdersPage{VendorName: vendorName, ScheduledOrders: scheduledOrders, Orders: orders}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor orders: execute failed: %v", err)
	}
}
