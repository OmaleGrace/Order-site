package handlers

import "log"

// topShops returns up to `limit` live shops for the home page, verified ones first.
func (h *Handlers) topShops(limit int) []ShopCard {
	rows, err := h.DB.Query(`
		SELECT v.name, v.slug, v.description, v.area, v.logo_url, v.verified,
		       v.offers_pickup, v.offers_delivery, v.delivery_fee_kobo,
		       (SELECT count(*) FROM menu_items m WHERE m.vendor_id = v.id) AS dishes,
		       (SELECT count(*) FROM orders o WHERE o.vendor_id = v.id AND o.status = 'completed') AS completed
		FROM vendors v
		WHERE v.status = 'active' AND v.paid_until > NOW()
		  AND EXISTS (SELECT 1 FROM menu_items m WHERE m.vendor_id = v.id)
		ORDER BY v.verified DESC, completed DESC, v.name
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
