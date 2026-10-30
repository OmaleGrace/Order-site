package handlers

import (
	"testing"

	"Order-site/testutil"
)

// These tests run against the real schema, so they also prove that the
// vendor_delivery_rates table exists and that the queries match its columns.

func TestResolveDeliveryFeeUsesAreaPricing(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()

	var vendorID int
	if err := db.QueryRow(`INSERT INTO vendors (name, slug) VALUES ('Mama Put', 'mama-put') RETURNING id`).Scan(&vendorID); err != nil {
		t.Fatal(err)
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Ikeja: own fee of ₦1,500, minimum order ₦3,000, free delivery from ₦10,000.
	mustExec(`INSERT INTO vendor_delivery_rates (vendor_id, state, lga, fee_kobo, min_order_kobo, free_over_kobo)
	          VALUES ($1, 'Lagos', 'Ikeja', 150000, 300000, 1000000)`, vendorID)
	// Yaba: no fee of its own, so the shop's default applies; no minimum.
	mustExec(`INSERT INTO vendor_delivery_rates (vendor_id, state, lga, fee_kobo) VALUES ($1, 'Lagos', 'Yaba', NULL)`, vendorID)

	const defaultFee = 50000 // ₦500

	cases := []struct {
		name        string
		state, lga  string
		food        int
		wantFee     int
		wantBlocked bool
	}{
		{"area fee replaces the default", "Lagos", "Ikeja", 500000, 150000, false},
		{"matching ignores case and spaces", " lagos ", "IKEJA", 500000, 150000, false},
		{"below the minimum order is blocked", "Lagos", "Ikeja", 299999, 150000, true},
		{"free delivery above the threshold", "Lagos", "Ikeja", 1000000, 0, false},
		{"blank area fee keeps the shop default", "Lagos", "Yaba", 100000, defaultFee, false},
		{"unpriced area keeps the shop default", "Lagos", "Surulere", 100000, defaultFee, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fee, blocked, err := resolveDeliveryFee(db, vendorID, c.state, c.lga, c.food, defaultFee)
			if err != nil {
				t.Fatal(err)
			}
			if (blocked != "") != c.wantBlocked {
				t.Fatalf("blocked = %q, want blocked=%v", blocked, c.wantBlocked)
			}
			if !c.wantBlocked && fee != c.wantFee {
				t.Fatalf("fee = %d, want %d", fee, c.wantFee)
			}
		})
	}
}

func TestLoadCheckoutRatesOnlyForTheCartsShop(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()

	var userID, shopA, shopB, dish int
	for _, q := range []struct {
		sql string
		dst *int
	}{
		{`INSERT INTO users (name, email, password) VALUES ('Ada', 'ada@example.com', 'x') RETURNING id`, &userID},
		{`INSERT INTO vendors (name, slug) VALUES ('Shop A', 'shop-a') RETURNING id`, &shopA},
		{`INSERT INTO vendors (name, slug) VALUES ('Shop B', 'shop-b') RETURNING id`, &shopB},
	} {
		if err := db.QueryRow(q.sql).Scan(q.dst); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRow(`INSERT INTO menu_items (name, price_kobo, vendor_id) VALUES ('Jollof', 150000, $1) RETURNING id`, shopA).Scan(&dish); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO cart_items (user_id, menu_item_id, quantity, price_kobo_at_addition) VALUES ($1, $2, 1, 150000)`, userID, dish); err != nil {
		t.Fatal(err)
	}
	for shop, area := range map[int][]any{shopA: {"Ikeja", 100000}, shopB: {"Yaba", 200000}} {
		if _, err := db.Exec(`INSERT INTO vendor_delivery_rates (vendor_id, state, lga, fee_kobo) VALUES ($1, 'Lagos', $2, $3)`, shop, area[0], area[1]); err != nil {
			t.Fatal(err)
		}
	}

	rates, err := loadCheckoutRates(db, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 1 || rates[0].LGA != "Ikeja" || rates[0].FeeKobo == nil || *rates[0].FeeKobo != 100000 {
		t.Fatalf("expected only Shop A's Ikeja rate, got %+v", rates)
	}
}
