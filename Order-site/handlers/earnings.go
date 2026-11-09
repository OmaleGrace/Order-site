package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Order-site/errors"
	"Order-site/render"
)

// VendorEarnings adds up a shop's sales. Only paid orders count (not pending or cancelled).
type VendorEarnings struct {
	Orders         int
	SalesKobo      int // everything customers paid, delivery fees included
	CommissionKobo int // the platform's cut
	PaidOutKobo    int // already sent to the vendor
	BalanceKobo    int // still owed to the vendor
	AvailableKobo  int // part of the balance that can be sent now
	HeldKobo       int // completed orders still inside the hold period
	InProgressKobo int // paid orders that are not completed yet
	HoldHours      int
}

type VendorPayoutRow struct {
	AmountKobo int
	Note       string
	PaidAt     string
	Status     string // paid, processing
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
		"SELECT COALESCE(SUM(amount_kobo), 0) FROM vendor_payouts WHERE vendor_id = $1 AND status <> 'failed'", vendorID,
	).Scan(&e.PaidOutKobo)
	if err != nil {
		return e, err
	}
	e.BalanceKobo = e.SalesKobo - e.CommissionKobo - e.PaidOutKobo
	if pos, perr := loadPayoutPosition(context.Background(), h.DB, vendorID); perr == nil {
		e.AvailableKobo, e.HeldKobo, e.InProgressKobo = pos.AvailableKobo, pos.HeldKobo, pos.InProgressKobo
	} else {
		log.Printf("earnings: payout position failed for vendor %d: %v", vendorID, perr)
	}
	e.HoldHours = payoutHoldHours()
	return e, nil
}

func (h *Handlers) vendorPayouts(vendorID, limit int) []VendorPayoutRow {
	rows, err := h.DB.Query(`
		SELECT amount_kobo, note, TO_CHAR(paid_at, 'DD Mon YYYY'), status
		FROM vendor_payouts WHERE vendor_id = $1 AND status <> 'failed'
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
		if err := rows.Scan(&p.AmountKobo, &p.Note, &p.PaidAt, &p.Status); err == nil {
			out = append(out, p)
		}
	}
	return out
}

type AdminEarningsRow struct {
	VendorID        int
	Name            string
	BankName        string
	BankCode        string
	AccountNumber   string
	AccountName     string
	AccountVerified bool
	VendorEarnings
}

type AdminEarningsPage struct {
	Rows              []AdminEarningsRow
	Notice            string
	CommissionPercent int
	TotalSalesKobo    int
	TotalCommission   int
	TotalServiceFees  int
	WalletHeldKobo    int // customer wallet balances: money you owe back to customers
	TotalOwedKobo     int
	TotalAvailable    int
	HoldHours         int
}

// AdminEarnings shows, per vendor, what was sold, what you keep, and what you still owe them.
func (h *Handlers) AdminEarnings(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`
		SELECT v.id, v.name, COALESCE(v.payout_bank_name,''), COALESCE(v.payout_bank_code,''),
		       COALESCE(v.payout_account_number,''), COALESCE(v.payout_account_name,''), COALESCE(v.payout_account_verified,FALSE),
		       COALESCE(o.cnt, 0), COALESCE(o.sales, 0), COALESCE(o.comm, 0), COALESCE(p.paid, 0)
		FROM vendors v
		LEFT JOIN (
			SELECT vendor_id, COUNT(*) AS cnt, SUM(total_kobo) AS sales, SUM(commission_kobo) AS comm
			FROM orders WHERE status NOT IN ('pending', 'cancelled') GROUP BY vendor_id
		) o ON o.vendor_id = v.id
		LEFT JOIN (
			SELECT vendor_id, SUM(amount_kobo) AS paid FROM vendor_payouts WHERE status <> 'failed' GROUP BY vendor_id
		) p ON p.vendor_id = v.id
		ORDER BY v.id
	`)
	if err != nil {
		log.Printf("admin earnings: query failed: %v", err)
		errors.Render(w, "Could not load earnings", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	page := AdminEarningsPage{Notice: r.URL.Query().Get("msg"), CommissionPercent: commissionPercent(), HoldHours: payoutHoldHours()}
	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(amount_kobo), 0) FROM wallet_transactions`).Scan(&page.WalletHeldKobo)
	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(service_fee_kobo), 0) FROM orders WHERE status NOT IN ('pending', 'cancelled')`).Scan(&page.TotalServiceFees)
	for rows.Next() {
		var row AdminEarningsRow
		if err := rows.Scan(&row.VendorID, &row.Name, &row.BankName, &row.BankCode, &row.AccountNumber, &row.AccountName, &row.AccountVerified, &row.Orders, &row.SalesKobo, &row.CommissionKobo, &row.PaidOutKobo); err != nil {
			log.Printf("admin earnings: scan failed: %v", err)
			errors.Render(w, "Could not read earnings", http.StatusInternalServerError)
			return
		}
		row.BalanceKobo = row.SalesKobo - row.CommissionKobo - row.PaidOutKobo
		if pos, perr := loadPayoutPosition(r.Context(), h.DB, row.VendorID); perr == nil {
			row.AvailableKobo, row.HeldKobo, row.InProgressKobo = pos.AvailableKobo, pos.HeldKobo, pos.InProgressKobo
		}
		page.TotalAvailable += row.AvailableKobo
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

	tmpl, err := render.Page("admin-earnings.html", template.FuncMap{"money": money, "money2": money2})
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

	var verified bool
	if err := tx.QueryRow(`SELECT payout_account_verified FROM vendors WHERE id=$1`, vendorID).Scan(&verified); err != nil {
		log.Printf("admin payout: account verification lookup failed: %v", err)
		errors.Render(w, "Could not verify vendor payout account", http.StatusInternalServerError)
		return
	}
	if !verified {
		back("The vendor must verify their payout bank account before a payout can be recorded.")
		return
	}

	pos, err := loadPayoutPosition(r.Context(), tx, vendorID)
	if err != nil {
		log.Printf("admin payout: balance failed: %v", err)
		errors.Render(w, "Could not record the payout", http.StatusInternalServerError)
		return
	}
	if amount > pos.AvailableKobo {
		back("Only ₦" + money(pos.AvailableKobo) + " can be paid out now. The rest is on hold for " + strconv.Itoa(payoutHoldHours()) + " hours after each order is completed, or the orders are not completed yet.")
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
