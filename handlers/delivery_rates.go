package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	appErrors "Order-site/errors"
	"Order-site/render"
)

// Delivery pricing per area (Stage 23).
//
// A shop already lists the states and LGAs it delivers to (vendor_delivery_areas)
// and has one default delivery fee. vendor_delivery_rates lets it refine that for
// each area: its own fee, a minimum order, and free delivery above an amount.
// Rates live in their own table because saving the shop profile rewrites
// vendor_delivery_areas, which would otherwise wipe the prices.

// CheckoutRate is one area's delivery pricing, as the checkout page needs it.
type CheckoutRate struct {
	State        string `json:"state"`
	LGA          string `json:"lga"`
	FeeKobo      *int   `json:"fee_kobo"` // nil means "use the shop's default fee"
	MinOrderKobo int    `json:"min_order_kobo"`
	FreeOverKobo int    `json:"free_over_kobo"`
}

// DeliveryRateRow is one editable row on the vendor's pricing page (amounts in naira).
type DeliveryRateRow struct {
	State    string
	LGA      string
	Fee      string // empty = use the shop default
	MinOrder string // empty = no minimum
	FreeOver string // empty = never free
}

type VendorDeliveryPage struct {
	VendorName     string
	Notice         string
	OffersDelivery bool
	DefaultFee     string
	Rows           []DeliveryRateRow
}

func areaKey(state, lga string) string {
	return strings.ToLower(strings.TrimSpace(state)) + "|" + strings.ToLower(strings.TrimSpace(lga))
}

// formatNaira turns kobo into a friendly amount, e.g. 1250000 -> ₦12,500.
func formatNaira(kobo int) string {
	digits := strconv.Itoa(kobo / 100)
	var b strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return "₦" + b.String()
}

// nairaText shows kobo as an editable naira amount: 150000 -> "1500", 150050 -> "1500.50".
func nairaText(kobo int) string {
	if kobo%100 == 0 {
		return strconv.Itoa(kobo / 100)
	}
	return fmt.Sprintf("%.2f", float64(kobo)/100)
}

// nairaToKobo reads a naira amount typed by a vendor. A blank field is reported as
// blank (not an error) so "no value" can be told apart from zero.
func nairaToKobo(text string, maxNaira int) (kobo int, blank bool, ok bool) {
	text = strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(text), ",", ""), "₦")
	if text == "" {
		return 0, true, true
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > float64(maxNaira) {
		return 0, false, false
	}
	return int(math.Round(value * 100)), false, true
}

// applyDeliveryRate applies one area's rules to an order. It returns the delivery
// fee, or a customer-facing reason when the order cannot be delivered to that area.
// A nil rate means the shop has not priced the area, so its default fee applies.
func applyDeliveryRate(rate *CheckoutRate, foodKobo, defaultFeeKobo int) (feeKobo int, blocked string) {
	feeKobo = defaultFeeKobo
	if rate == nil {
		return feeKobo, ""
	}
	if rate.FeeKobo != nil {
		feeKobo = *rate.FeeKobo
	}
	if rate.MinOrderKobo > 0 && foodKobo < rate.MinOrderKobo {
		return feeKobo, fmt.Sprintf("The minimum order for delivery to %s is %s. Add more items or choose pickup.",
			rate.LGA, formatNaira(rate.MinOrderKobo))
	}
	if rate.FreeOverKobo > 0 && foodKobo >= rate.FreeOverKobo {
		feeKobo = 0
	}
	return feeKobo, ""
}

// resolveDeliveryFee looks up the shop's pricing for the customer's area. The server
// always decides the fee; the checkout page only previews it.
func resolveDeliveryFee(db *sql.DB, vendorID int, state, lga string, foodKobo, defaultFeeKobo int) (int, string, error) {
	var rate CheckoutRate
	var fee sql.NullInt64
	err := db.QueryRow(`
		SELECT state, lga, fee_kobo, min_order_kobo, free_over_kobo
		FROM vendor_delivery_rates
		WHERE vendor_id = $1
		  AND LOWER(TRIM(state)) = LOWER(TRIM($2))
		  AND LOWER(TRIM(lga)) = LOWER(TRIM($3))
	`, vendorID, state, lga).Scan(&rate.State, &rate.LGA, &fee, &rate.MinOrderKobo, &rate.FreeOverKobo)
	if err == sql.ErrNoRows {
		return defaultFeeKobo, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	if fee.Valid {
		v := int(fee.Int64)
		rate.FeeKobo = &v
	}
	feeKobo, blocked := applyDeliveryRate(&rate, foodKobo, defaultFeeKobo)
	return feeKobo, blocked, nil
}

// loadCheckoutRates returns the pricing of the shop in this customer's cart.
func loadCheckoutRates(db *sql.DB, userID interface{}) ([]CheckoutRate, error) {
	rows, err := db.Query(`
		SELECT r.state, r.lga, r.fee_kobo, r.min_order_kobo, r.free_over_kobo
		FROM vendor_delivery_rates r
		WHERE r.vendor_id IN (
			SELECT DISTINCT m.vendor_id
			FROM cart_items ci
			JOIN menu_items m ON m.id = ci.menu_item_id
			WHERE ci.user_id = $1
		)
		ORDER BY r.state, r.lga
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []CheckoutRate
	for rows.Next() {
		var r CheckoutRate
		var fee sql.NullInt64
		if err := rows.Scan(&r.State, &r.LGA, &fee, &r.MinOrderKobo, &r.FreeOverKobo); err != nil {
			return nil, err
		}
		if fee.Valid {
			v := int(fee.Int64)
			r.FeeKobo = &v
		}
		rates = append(rates, r)
	}
	return rates, rows.Err()
}

// VendorDeliveryPricing shows the vendor's areas with editable pricing.
func (h *Handlers) VendorDeliveryPricing(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	page := VendorDeliveryPage{VendorName: vendorName, Notice: r.URL.Query().Get("msg")}

	var defaultFee int
	if err := h.DB.QueryRow(`SELECT offers_delivery, delivery_fee_kobo FROM vendors WHERE id = $1`, vendorID).
		Scan(&page.OffersDelivery, &defaultFee); err != nil {
		log.Printf("vendor delivery: shop query failed: %v", err)
		appErrors.Render(w, "Could not load delivery pricing", http.StatusInternalServerError)
		return
	}
	page.DefaultFee = formatNaira(defaultFee)

	rows, err := h.DB.Query(`
		SELECT a.state, a.lga, r.fee_kobo, COALESCE(r.min_order_kobo, 0), COALESCE(r.free_over_kobo, 0)
		FROM vendor_delivery_areas a
		LEFT JOIN vendor_delivery_rates r
		  ON r.vendor_id = a.vendor_id
		 AND LOWER(TRIM(r.state)) = LOWER(TRIM(a.state))
		 AND LOWER(TRIM(r.lga)) = LOWER(TRIM(a.lga))
		WHERE a.vendor_id = $1
		ORDER BY a.state, a.lga
	`, vendorID)
	if err != nil {
		log.Printf("vendor delivery: areas query failed: %v", err)
		appErrors.Render(w, "Could not load delivery areas", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var row DeliveryRateRow
		var fee sql.NullInt64
		var minKobo, freeKobo int
		if err := rows.Scan(&row.State, &row.LGA, &fee, &minKobo, &freeKobo); err != nil {
			log.Printf("vendor delivery: areas scan failed: %v", err)
			appErrors.Render(w, "Could not read delivery areas", http.StatusInternalServerError)
			return
		}
		if fee.Valid {
			row.Fee = nairaText(int(fee.Int64))
		}
		if minKobo > 0 {
			row.MinOrder = nairaText(minKobo)
		}
		if freeKobo > 0 {
			row.FreeOver = nairaText(freeKobo)
		}
		page.Rows = append(page.Rows, row)
	}
	if err := rows.Err(); err != nil {
		log.Printf("vendor delivery: areas rows failed: %v", err)
		appErrors.Render(w, "Could not read delivery areas", http.StatusInternalServerError)
		return
	}

	tmpl, err := render.Page("vendor-delivery.html")
	if err != nil {
		log.Printf("vendor delivery: template failed: %v", err)
		appErrors.Render(w, "Could not display delivery pricing", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor delivery: execute failed: %v", err)
	}
}

// VendorDeliverySave stores the pricing the vendor entered. Only areas the shop
// actually serves are accepted, whatever the browser sends.
func (h *Handlers) VendorDeliverySave(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	back := func(message string) {
		http.Redirect(w, r, "/vendor/delivery?msg="+url.QueryEscape(message), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		back("Could not read the form. Please try again.")
		return
	}
	states, lgas := r.Form["state"], r.Form["lga"]
	fees, mins, frees := r.Form["fee"], r.Form["min_order"], r.Form["free_over"]
	count := len(states)
	if len(lgas) != count || len(fees) != count || len(mins) != count || len(frees) != count {
		back("The form was incomplete. Please try again.")
		return
	}

	served := map[string][2]string{}
	areaRows, err := h.DB.Query(`SELECT state, lga FROM vendor_delivery_areas WHERE vendor_id = $1`, vendorID)
	if err != nil {
		log.Printf("vendor delivery save: areas query failed: %v", err)
		back("Could not save delivery pricing. Please try again.")
		return
	}
	for areaRows.Next() {
		var state, lga string
		if err := areaRows.Scan(&state, &lga); err != nil {
			areaRows.Close()
			log.Printf("vendor delivery save: areas scan failed: %v", err)
			back("Could not save delivery pricing. Please try again.")
			return
		}
		served[areaKey(state, lga)] = [2]string{state, lga}
	}
	areaErr := areaRows.Err()
	areaRows.Close()
	if areaErr != nil {
		log.Printf("vendor delivery save: areas rows failed: %v", areaErr)
		back("Could not save delivery pricing. Please try again.")
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		log.Printf("vendor delivery save: begin failed: %v", err)
		back("Could not save delivery pricing. Please try again.")
		return
	}
	defer tx.Rollback()

	for i := 0; i < count; i++ {
		area, found := served[areaKey(states[i], lgas[i])]
		if !found {
			continue
		}
		feeKobo, feeBlank, feeOK := nairaToKobo(fees[i], 100000)
		minKobo, _, minOK := nairaToKobo(mins[i], 10000000)
		freeKobo, _, freeOK := nairaToKobo(frees[i], 10000000)
		if !feeOK || !minOK || !freeOK {
			back(fmt.Sprintf("Please check the amounts for %s. Use numbers like 1500 or 1500.50.", area[1]))
			return
		}

		if feeBlank && minKobo == 0 && freeKobo == 0 {
			if _, err := tx.Exec(`DELETE FROM vendor_delivery_rates WHERE vendor_id = $1 AND state = $2 AND lga = $3`,
				vendorID, area[0], area[1]); err != nil {
				log.Printf("vendor delivery save: delete failed: %v", err)
				back("Could not save delivery pricing. Please try again.")
				return
			}
			continue
		}

		var feeArg interface{} // nil is stored as NULL, meaning "use the shop's default fee"
		if !feeBlank {
			feeArg = feeKobo
		}
		if _, err := tx.Exec(`
			INSERT INTO vendor_delivery_rates (vendor_id, state, lga, fee_kobo, min_order_kobo, free_over_kobo, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (vendor_id, state, lga) DO UPDATE
			SET fee_kobo = EXCLUDED.fee_kobo,
			    min_order_kobo = EXCLUDED.min_order_kobo,
			    free_over_kobo = EXCLUDED.free_over_kobo,
			    updated_at = NOW()
		`, vendorID, area[0], area[1], feeArg, minKobo, freeKobo); err != nil {
			log.Printf("vendor delivery save: upsert failed: %v", err)
			back("Could not save delivery pricing. Please try again.")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("vendor delivery save: commit failed: %v", err)
		back("Could not save delivery pricing. Please try again.")
		return
	}
	back("Delivery pricing saved.")
}
