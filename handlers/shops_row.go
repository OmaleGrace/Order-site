package handlers

import "log"

// topShops returns up to `limit` live shops for the home page, verified ones first.
func (h *Handlers) topShops(limit int) []ShopCard {
	rows, err := h.DB.Query(`
		SELECT v.name, v.slug, v.description, v.area, v.logo_url, v.verified,
		       v.offers_pickup, v.offers_delivery, v.delivery_fee_kobo,
		       COALESCE(mi.dishes, 0) AS dishes,
		       COALESCE(oc.completed, 0) AS completed
		FROM vendors v
		JOIN (
			SELECT vendor_id, COUNT(*) AS dishes
			FROM menu_items
			GROUP BY vendor_id
		) mi ON mi.vendor_id = v.id
		LEFT JOIN (
			SELECT vendor_id, COUNT(*) AS completed
			FROM orders
			WHERE status = 'completed'
			GROUP BY vendor_id
		) oc ON oc.vendor_id = v.id
		WHERE v.status = 'active' AND v.paid_until > NOW()
		ORDER BY v.verified DESC, COALESCE(oc.completed, 0) DESC, v.name
		LIMIT $1
	`, limit)
	if err != nil {
		log.Printf("home: shops query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var shops []ShopCard
	for rows.Next() {
		var s ShopCard
		if err := rows.Scan(&s.Name, &s.Slug, &s.Description, &s.Area, &s.LogoURL, &s.Verified,
			&s.OffersPickup, &s.OffersDelivery, &s.DeliveryFeeKobo, &s.Dishes, &s.Completed); err != nil {
			log.Printf("home: shops scan failed: %v", err)
			continue
		}
		shops = append(shops, s)
	}
	return shops
}
