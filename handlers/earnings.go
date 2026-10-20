package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
)

// VendorEarnings adds up a shop's sales. Only paid orders count (not pending or cancelled).
type VendorEarnings struct {
	Orders         int
	SalesKobo      int // everything customers paid, delivery fees included
	CommissionKobo int // the platform's cut
	PaidOutKobo    int // already sent to the vendor
	BalanceKobo    int // still owed to the vendor
}

type VendorPayoutRow struct {
	AmountKobo int
	Note       string
	PaidAt     string
}

func (h *Handlers) vendorEarnings(vendorID int) (VendorEarnings, error) {
	var e VendorEarnings
	err := h.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(total_kobo), 0), COALESCE(SUM(commission_kobo), 0)
		FROM orders
		WHERE vendor_id = $1 AND status NOT IN ('pending', 'cancelled')
	`, vendorID).Scan(&e.Orders, &e.SalesKobo, &e.CommissionKobo)
	if err != nil {
		return e, err
	}
	err = h.DB.QueryRow(
		"SELECT COALESCE(SUM(amount_kobo), 0) FROM vendor_payouts WHERE vendor_id = $1", vendorID,
	).Scan(&e.PaidOutKobo)
	if err != nil {
		return e, err
	}
	e.BalanceKobo = e.SalesKobo - e.CommissionKobo - e.PaidOutKobo
	return e, nil
}

func (h *Handlers) vendorPayouts(vendorID, limit int) []VendorPayoutRow {
	rows, err := h.DB.Query(`
		SELECT amount_kobo, note, TO_CHAR(paid_at, 'DD Mon YYYY')
		FROM vendor_payouts WHERE vendor_id = $1
		ORDER BY paid_at DESC, id DESC LIMIT $2
	`, vendorID, limit)
	if err != nil {
		log.Printf("payouts: query failed: %v", err)
		return nil
	}
	defer rows.Close()
	var out []VendorPayoutRow
	for rows.Next() {
		var p VendorPayoutRow
		if err := rows.Scan(&p.AmountKobo, &p.Note, &p.PaidAt); err == nil {
			out = append(out, p)
		}
	}
	return out
}

type AdminEarningsRow struct {
	VendorID int
	Name     string
	VendorEarnings
}

type AdminEarningsPage struct {
	Rows              []AdminEarningsRow
	Notice            string
	CommissionPercent int
	TotalSalesKobo    int
	TotalCommission   int
	TotalOwedKobo     int
}

// AdminEarnings shows, per vendor, what was sold, what you keep, and what you still owe them.
func (h *Handlers) AdminEarnings(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`
		SELECT v.id, v.name,
		       COALESCE(o.cnt, 0), COALESCE(o.sales, 0), COALESCE(o.comm, 0), COALESCE(p.paid, 0)
		FROM vendors v
		LEFT JOIN (
			SELECT vendor_id, COUNT(*) AS cnt, SUM(total_kobo) AS sales, SUM(commission_kobo) AS comm
			FROM orders WHERE status NOT IN ('pending', 'cancelled') GROUP BY vendor_id
		) o ON o.vendor_id = v.id
		LEFT JOIN (
			SELECT vendor_id, SUM(amount_kobo) AS paid FROM vendor_payouts GROUP BY vendor_id
		) p ON p.vendor_id = v.id
		ORDER BY v.id
	`)
	if err != nil {
		log.Printf("admin earnings: query failed: %v", err)
		errors.Render(w, "Could not load earnings", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	page := AdminEarningsPage{Notice: r.URL.Query().Get("msg"), CommissionPercent: commissionPercent()}
	for rows.Next() {
		var row AdminEarningsRow
		if err := rows.Scan(&row.VendorID, &row.Name, &row.Orders, &row.SalesKobo, &row.CommissionKobo, &row.PaidOutKobo); err != nil {
			log.Printf("admin earnings: scan failed: %v", err)
			errors.Render(w, "Could not read earnings", http.StatusInternalServerError)
			return
		}
		row.BalanceKobo = row.SalesKobo - row.CommissionKobo - row.PaidOutKobo
		page.TotalSalesKobo += row.SalesKobo
		page.TotalCommission += row.CommissionKobo
		page.TotalOwedKobo += row.BalanceKobo
		page.Rows = append(page.Rows, row)
	}
	if err := rows.Err(); err != nil {
		log.Printf("admin earnings: rows failed: %v", err)
		errors.Render(w, "Could not read earnings", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("admin-earnings.html").
		Funcs(template.FuncMap{"money": money, "money2": money2}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/admin-earnings.html")
	if err != nil {
		log.Printf("admin earnings: template failed: %v", err)
		errors.Render(w, "Could not display earnings", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("admin earnings: execute failed: %v", err)
	}
}

// money shows kobo as naira with two decimals, for example 1,250.50
func money(kobo int) string {
	neg := kobo < 0
	if neg {
		kobo = -kobo
	}
	whole := strconv.Itoa(kobo / 100)
	var b strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	s := fmt.Sprintf("%s.%02d", b.String(), kobo%100)
	if neg {
		return "-" + s
	}
	return s
}

// money2 is like money but without thousands commas, for use inside form fields.
func money2(kobo int) string { return fmt.Sprintf("%d.%02d", kobo/100, kobo%100) }

// AdminRecordPayout notes that you have sent a vendor some of the money they are owed.
// It does not move any money: you send the transfer yourself, then record it here.
func (h *Handlers) AdminRecordPayout(w http.ResponseWriter, r *http.Request) {
	adminID, _ := h.sessionUserID(r)
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}
	back := func(msg string) {
		http.Redirect(w, r, "/admin/earnings?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	vendorID, err := strconv.Atoi(r.FormValue("vendor_id"))
	if err != nil {
		errors.Render(w, "Invalid vendor", http.StatusBadRequest)
		return
	}
	naira, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("amount")), 64)
	if err != nil || naira <= 0 || naira > 100000000 {
		back("Enter a valid amount in naira.")
		return
	}
	amount := int(math.Round(naira * 100))
	note := strings.TrimSpace(r.FormValue("note"))
	if len([]rune(note)) > 200 {
		back("The note must be 200 characters or fewer.")
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		errors.Render(w, "Could not record the payout", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Lock the vendor so two payouts at once cannot both pass the balance check
	var exists int
	if err := tx.QueryRow("SELECT id FROM vendors WHERE id = $1 FOR UPDATE", vendorID).Scan(&exists); err == sql.ErrNoRows {
		back("That vendor was not found.")
		return
	} else if err != nil {
		log.Printf("admin payout: lock failed: %v", err)
		errors.Render(w, "Could not record the payout", http.StatusInternalServerError)
		return
	}

	var sales, comm, paid int
	err = tx.QueryRow(`
		SELECT COALESCE(SUM(total_kobo), 0), COALESCE(SUM(commission_kobo), 0)
		FROM orders WHERE vendor_id = $1 AND status NOT IN ('pending', 'cancelled')
	`, vendorID).Scan(&sales, &comm)
	if err == nil {
		err = tx.QueryRow("SELECT COALESCE(SUM(amount_kobo), 0) FROM vendor_payouts WHERE vendor_id = $1", vendorID).Scan(&paid)
	}
	if err != nil {
		log.Printf("admin payout: balance failed: %v", err)
		errors.Render(w, "Could not record the payout", http.StatusInternalServerError)
		return
	}
	if amount > sales-comm-paid {
		back("That is more than this vendor is owed (₦" + money(sales-comm-paid) + ").")
		return
	}

	if _, err := tx.Exec(`
		INSERT INTO vendor_payouts (vendor_id, amount_kobo, note, recorded_by)
		VALUES ($1, $2, $3, $4)
	`, vendorID, amount, note, adminID); err != nil {
		log.Printf("admin payout: insert failed: %v", err)
		errors.Render(w, "Could not record the payout", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("admin payout: commit failed: %v", err)
		errors.Render(w, "Could not record the payout", http.StatusInternalServerError)
		return
	}
	log.Printf("admin payout: admin=%d vendor=%d amount_kobo=%d", adminID, vendorID, amount)
	back("Payout recorded.")
}
