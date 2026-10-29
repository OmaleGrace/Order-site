package handlers

import (
	"encoding/csv"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"Order-site/brand"
	appErrors "Order-site/errors"
)

// AdminSettlementRow is one shop's position for a period: what customers paid, what
// ENIE keeps, what the shop earns, and what has been sent to it.
type AdminSettlementRow struct {
	VendorID        int
	Name            string
	PayoutVerified  bool
	Orders          int
	CollectedKobo   int // customers paid, delivery included
	DeliveryKobo    int // part of CollectedKobo
	CommissionKobo  int
	NetKobo         int // CollectedKobo - CommissionKobo
	PaidInKobo      int // payouts sent during the period
	OutstandingKobo int // still owed to the shop, across all time
}

type AdminSettlementTotals struct {
	Orders          int
	CollectedKobo   int
	DeliveryKobo    int
	CommissionKobo  int
	NetKobo         int
	PaidInKobo      int
	OutstandingKobo int
}

type AdminSettlementsPage struct {
	Notice    string
	Period    statementPeriod
	Choices   []periodChoice
	ExportURL template.URL
	Rows      []AdminSettlementRow
	Totals    AdminSettlementTotals
	Generated string
}

func sumSettlements(rows []AdminSettlementRow) AdminSettlementTotals {
	var t AdminSettlementTotals
	for _, r := range rows {
		t.Orders += r.Orders
		t.CollectedKobo += r.CollectedKobo
		t.DeliveryKobo += r.DeliveryKobo
		t.CommissionKobo += r.CommissionKobo
		t.NetKobo += r.NetKobo
		t.PaidInKobo += r.PaidInKobo
		t.OutstandingKobo += r.OutstandingKobo
	}
	return t
}

func (h *Handlers) loadSettlements(p statementPeriod) ([]AdminSettlementRow, error) {
	rows, err := h.DB.Query(`
		SELECT v.id, v.name, COALESCE(v.payout_account_verified, false),
		       COALESCE(o.cnt, 0), COALESCE(o.collected, 0), COALESCE(o.delivery, 0), COALESCE(o.comm, 0),
		       COALESCE(p.paid, 0),
		       COALESCE(lo.collected, 0) - COALESCE(lo.comm, 0) - COALESCE(lp.paid, 0)
		FROM vendors v
		LEFT JOIN (
			SELECT vendor_id, COUNT(*) AS cnt, SUM(total_kobo) AS collected,
			       SUM(delivery_fee_kobo) AS delivery, SUM(commission_kobo) AS comm
			FROM orders
			WHERE status NOT IN ('pending', 'cancelled')
			  AND created_at >= $1::timestamp AND created_at < $2::timestamp
			GROUP BY vendor_id
		) o ON o.vendor_id = v.id
		LEFT JOIN (
			SELECT vendor_id, SUM(amount_kobo) AS paid
			FROM vendor_payouts
			WHERE paid_at >= $3 AND paid_at < $4
			GROUP BY vendor_id
		) p ON p.vendor_id = v.id
		LEFT JOIN (
			SELECT vendor_id, SUM(total_kobo) AS collected, SUM(commission_kobo) AS comm
			FROM orders WHERE status NOT IN ('pending', 'cancelled')
			GROUP BY vendor_id
		) lo ON lo.vendor_id = v.id
		LEFT JOIN (
			SELECT vendor_id, SUM(amount_kobo) AS paid FROM vendor_payouts GROUP BY vendor_id
		) lp ON lp.vendor_id = v.id
		ORDER BY v.name
	`, naiveUTC(p.Start), naiveUTC(p.End), p.Start.UTC(), p.End.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AdminSettlementRow
	for rows.Next() {
		var r AdminSettlementRow
		if err := rows.Scan(&r.VendorID, &r.Name, &r.PayoutVerified, &r.Orders, &r.CollectedKobo,
			&r.DeliveryKobo, &r.CommissionKobo, &r.PaidInKobo, &r.OutstandingKobo); err != nil {
			return nil, err
		}
		r.NetKobo = r.CollectedKobo - r.CommissionKobo
		// Leave out shops with nothing to report, so the table stays readable.
		if r.Orders == 0 && r.PaidInKobo == 0 && r.OutstandingKobo == 0 {
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AdminSettlements is the platform-wide settlement report for a period.
func (h *Handlers) AdminSettlements(w http.ResponseWriter, r *http.Request) {
	period, problem := parseStatementPeriod(r.URL.Query(), time.Now())
	rows, err := h.loadSettlements(period)
	if err != nil {
		log.Printf("admin settlements: load failed: %v", err)
		appErrors.Render(w, "Could not load the settlement report", http.StatusInternalServerError)
		return
	}
	page := AdminSettlementsPage{
		Notice:    problem,
		Period:    period,
		Choices:   periodChoices(period.Preset),
		ExportURL: period.exportURL("/admin/settlements/export"),
		Rows:      rows,
		Totals:    sumSettlements(rows),
		Generated: time.Now().In(watZone).Format("2 Jan 2006, 3:04 PM"),
	}
	tmpl, err := template.New("admin-settlements.html").Funcs(statementFuncs()).Funcs(brand.Funcs()).ParseFiles("templates/admin-settlements.html")
	if err != nil {
		log.Printf("admin settlements: template failed: %v", err)
		appErrors.Render(w, "Could not display the settlement report", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("admin settlements: execute failed: %v", err)
	}
}

// AdminSettlementsExport sends the same report as a CSV file.
func (h *Handlers) AdminSettlementsExport(w http.ResponseWriter, r *http.Request) {
	period, _ := parseStatementPeriod(r.URL.Query(), time.Now())
	rows, err := h.loadSettlements(period)
	if err != nil {
		log.Printf("admin settlements export: load failed: %v", err)
		appErrors.Render(w, "Could not export the settlement report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+statementFilename("enie-settlements", period)+`"`)
	w.Header().Set("Cache-Control", "no-store")

	cw := csv.NewWriter(w)
	cw.Write([]string{"ENIE settlement report"})
	cw.Write([]string{"Period", period.Label})
	cw.Write([]string{"Generated", time.Now().In(watZone).Format("2006-01-02 15:04") + " WAT"})
	cw.Write([]string{"Amounts are in naira (NGN). Outstanding is the balance owed to the shop across all time."})
	cw.Write(nil)
	cw.Write([]string{"Shop", "Payout account verified", "Orders", "Customers paid", "of which delivery",
		"ENIE commission", "Shop earnings", "Paid out in period", "Outstanding"})
	for _, row := range rows {
		verified := "no"
		if row.PayoutVerified {
			verified = "yes"
		}
		cw.Write([]string{csvCell(row.Name), verified, strconv.Itoa(row.Orders), csvMoney(row.CollectedKobo),
			csvMoney(row.DeliveryKobo), csvMoney(row.CommissionKobo), csvMoney(row.NetKobo),
			csvMoney(row.PaidInKobo), csvMoney(row.OutstandingKobo)})
	}
	t := sumSettlements(rows)
	cw.Write([]string{"Totals", "", strconv.Itoa(t.Orders), csvMoney(t.CollectedKobo), csvMoney(t.DeliveryKobo),
		csvMoney(t.CommissionKobo), csvMoney(t.NetKobo), csvMoney(t.PaidInKobo), csvMoney(t.OutstandingKobo)})
	cw.Flush()
	if err := cw.Error(); err != nil {
		log.Printf("admin settlements export: write failed: %v", err)
	}
}
