package handlers

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appErrors "Order-site/errors"
	"Order-site/render"
)

// Vendor statements and settlement reports (Stage 29).
//
// These read the same orders and payouts the earnings pages use, with the same rule
// (an order counts once it is paid and not cancelled), so every figure reconciles:
//
//	customer paid  = food - coupon - loyalty + delivery
//	vendor earns   = customer paid - ENIE commission
//	closing        = opening balance + earnings - payouts

// Nigeria has no daylight saving, so a fixed UTC+1 offset is exact and does not
// depend on timezone data being installed on the server.
var watZone = time.FixedZone("WAT", 60*60)

const (
	maxStatementDays    = 1096  // longest custom range, about three years
	statementRowCap     = 20000 // refuse to build a statement bigger than this
	statementDisplayCap = 300   // orders shown on screen; the CSV always has them all
)

var errStatementTooLarge = fmt.Errorf("statement too large")

// statementPeriod is the date range of a statement. End is exclusive.
type statementPeriod struct {
	Start, End time.Time
	Label      string
	Preset     string
	From, To   string // YYYY-MM-DD, for the custom date boxes
	query      string // query string that reproduces this period
}

type periodChoice struct {
	Key    string
	Label  string
	Active bool
}

func periodChoices(active string) []periodChoice {
	choices := []periodChoice{
		{Key: "this-month", Label: "This month"},
		{Key: "last-month", Label: "Last month"},
		{Key: "last-3-months", Label: "Last 3 months"},
		{Key: "this-year", Label: "This year"},
		{Key: "all", Label: "All time"},
	}
	for i := range choices {
		choices[i].Active = choices[i].Key == active
	}
	return choices
}

// parseStatementPeriod reads ?period=... or ?from=...&to=... (dates are inclusive).
// When the dates are unusable it returns a reason, and the period falls back to this month.
func parseStatementPeriod(q url.Values, now time.Time) (statementPeriod, string) {
	now = now.In(watZone)
	monthStart := func(offset int) time.Time {
		return time.Date(now.Year(), now.Month()+time.Month(offset), 1, 0, 0, 0, 0, watZone)
	}

	preset := q.Get("period")
	fromText, toText := strings.TrimSpace(q.Get("from")), strings.TrimSpace(q.Get("to"))
	problem := ""
	if fromText != "" || toText != "" {
		from, errFrom := time.ParseInLocation("2006-01-02", fromText, watZone)
		to, errTo := time.ParseInLocation("2006-01-02", toText, watZone)
		switch {
		case errFrom != nil || errTo != nil:
			problem = "Please choose both a start date and an end date."
		case to.Before(from):
			problem = "The end date cannot be before the start date."
		case to.Sub(from) > time.Duration(maxStatementDays)*24*time.Hour:
			problem = "Please choose a range of three years or less."
		default:
			return statementPeriod{
				Start:  from,
				End:    to.AddDate(0, 0, 1),
				Label:  from.Format("2 Jan 2006") + " to " + to.Format("2 Jan 2006"),
				Preset: "custom",
				From:   fromText,
				To:     toText,
				query:  "from=" + url.QueryEscape(fromText) + "&to=" + url.QueryEscape(toText),
			}, ""
		}
		preset = ""
	}

	var start, end time.Time
	var label string
	switch preset {
	case "last-month":
		start, end = monthStart(-1), monthStart(0)
		label = start.Format("January 2006")
	case "last-3-months":
		start, end = monthStart(-2), monthStart(1)
		label = start.Format("Jan 2006") + " to " + monthStart(0).Format("Jan 2006")
	case "this-year":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, watZone)
		end = time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, watZone)
		label = strconv.Itoa(now.Year())
	case "all":
		start = time.Date(2020, 1, 1, 0, 0, 0, 0, watZone)
		end = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, watZone)
		label = "All time"
	default:
		preset = "this-month"
		start, end = monthStart(0), monthStart(1)
		label = start.Format("January 2006")
	}
	return statementPeriod{Start: start, End: end, Label: label, Preset: preset, query: "period=" + preset}, problem
}

// exportURL is the CSV link for this period under the given path. It is built only from
// validated values, so it is safe to mark as a URL for the template.
func (p statementPeriod) exportURL(path string) template.URL {
	return template.URL(path + "?" + p.query)
}

// naiveUTC formats an instant the way the orders table stores times (UTC, no zone).
func naiveUTC(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") }

// StatementOrder is one order broken into the parts a vendor cares about.
type StatementOrder struct {
	ID             int
	CreatedAt      time.Time // shown in Nigerian time
	Method         string
	Status         string
	FoodKobo       int // menu total before any discount
	CouponKobo     int
	LoyaltyKobo    int
	DeliveryKobo   int
	CollectedKobo  int // what the customer paid
	CommissionKobo int
	NetKobo        int // what the vendor earns from this order
}

// newStatementOrder rebuilds the food total from what was stored at checkout:
// total = food - coupon - loyalty + delivery.
func newStatementOrder(id int, created time.Time, method, status string, total, delivery, commission, loyaltyPoints, coupon int) StatementOrder {
	loyalty := loyaltyPoints * 100 // 1 point = ₦1
	return StatementOrder{
		ID:             id,
		CreatedAt:      created.In(watZone),
		Method:         method,
		Status:         status,
		FoodKobo:       total - delivery + loyalty + coupon,
		CouponKobo:     coupon,
		LoyaltyKobo:    loyalty,
		DeliveryKobo:   delivery,
		CollectedKobo:  total,
		CommissionKobo: commission,
		NetKobo:        total - commission,
	}
}

type StatementTotals struct {
	Orders         int
	FoodKobo       int
	CouponKobo     int
	LoyaltyKobo    int
	DeliveryKobo   int
	CollectedKobo  int
	CommissionKobo int
	NetKobo        int
	OpeningKobo    int // owed to the vendor when the period began
	PayoutsKobo    int // sent to the vendor during the period
	ClosingKobo    int // owed to the vendor when the period ended
}

func summarizeStatement(orders []StatementOrder, payoutsKobo, openingKobo int) StatementTotals {
	t := StatementTotals{Orders: len(orders), PayoutsKobo: payoutsKobo, OpeningKobo: openingKobo}
	for _, o := range orders {
		t.FoodKobo += o.FoodKobo
		t.CouponKobo += o.CouponKobo
		t.LoyaltyKobo += o.LoyaltyKobo
		t.DeliveryKobo += o.DeliveryKobo
		t.CollectedKobo += o.CollectedKobo
		t.CommissionKobo += o.CommissionKobo
		t.NetKobo += o.NetKobo
	}
	t.ClosingKobo = openingKobo + t.NetKobo - payoutsKobo
	return t
}

// formatNaira2 shows kobo as ₦1,250.50, with the minus sign in front of the ₦ when negative.
func formatNaira2(kobo int) string {
	if kobo < 0 {
		return "-₦" + money(-kobo)
	}
	return "₦" + money(kobo)
}

// csvMoney is a plain number for spreadsheets: 125050 -> 1250.50
func csvMoney(kobo int) string {
	if kobo < 0 {
		return "-" + money2(-kobo)
	}
	return money2(kobo)
}

// csvCell stops spreadsheet programs from running text that starts like a formula.
func csvCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func statementFilename(prefix string, p statementPeriod) string {
	return fmt.Sprintf("%s-%s-to-%s.csv", prefix,
		p.Start.In(watZone).Format("2006-01-02"), p.End.AddDate(0, 0, -1).In(watZone).Format("2006-01-02"))
}

type StatementPayout struct {
	AmountKobo int
	Note       string
	PaidAt     time.Time
}

type VendorStatement struct {
	Orders  []StatementOrder
	Payouts []StatementPayout
	Totals  StatementTotals
}

func (h *Handlers) loadVendorStatement(vendorID int, p statementPeriod) (VendorStatement, error) {
	var st VendorStatement
	start, end := naiveUTC(p.Start), naiveUTC(p.End)

	rows, err := h.DB.Query(`
		SELECT id, created_at, delivery_method, status, total_kobo, delivery_fee_kobo,
		       commission_kobo, loyalty_points_redeemed, coupon_discount_kobo
		FROM orders
		WHERE vendor_id = $1 AND status NOT IN ('pending', 'cancelled')
		  AND created_at >= $2::timestamp AND created_at < $3::timestamp
		ORDER BY created_at, id
		LIMIT $4
	`, vendorID, start, end, statementRowCap+1)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, total, delivery, commission, points, coupon int
		var created time.Time
		var method, status string
		if err := rows.Scan(&id, &created, &method, &status, &total, &delivery, &commission, &points, &coupon); err != nil {
			return st, err
		}
		st.Orders = append(st.Orders, newStatementOrder(id, created, method, status, total, delivery, commission, points, coupon))
	}
	if err := rows.Err(); err != nil {
		return st, err
	}
	if len(st.Orders) > statementRowCap {
		return st, errStatementTooLarge
	}

	// What the vendor was owed when the period began: earnings before it, less payouts before it.
	var earnedBefore, paidBefore int
	if err := h.DB.QueryRow(`
		SELECT COALESCE(SUM(total_kobo - commission_kobo), 0)
		FROM orders
		WHERE vendor_id = $1 AND status NOT IN ('pending', 'cancelled') AND created_at < $2::timestamp
	`, vendorID, start).Scan(&earnedBefore); err != nil {
		return st, err
	}
	if err := h.DB.QueryRow(
		`SELECT COALESCE(SUM(amount_kobo), 0) FROM vendor_payouts WHERE vendor_id = $1 AND paid_at < $2`,
		vendorID, p.Start.UTC(),
	).Scan(&paidBefore); err != nil {
		return st, err
	}

	payoutRows, err := h.DB.Query(`
		SELECT amount_kobo, note, paid_at
		FROM vendor_payouts
		WHERE vendor_id = $1 AND paid_at >= $2 AND paid_at < $3
		ORDER BY paid_at, id
	`, vendorID, p.Start.UTC(), p.End.UTC())
	if err != nil {
		return st, err
	}
	defer payoutRows.Close()
	paidInPeriod := 0
	for payoutRows.Next() {
		var po StatementPayout
		if err := payoutRows.Scan(&po.AmountKobo, &po.Note, &po.PaidAt); err != nil {
			return st, err
		}
		po.PaidAt = po.PaidAt.In(watZone)
		paidInPeriod += po.AmountKobo
		st.Payouts = append(st.Payouts, po)
	}
	if err := payoutRows.Err(); err != nil {
		return st, err
	}

	st.Totals = summarizeStatement(st.Orders, paidInPeriod, earnedBefore-paidBefore)
	return st, nil
}

type VendorStatementPage struct {
	VendorName        string
	Notice            string
	Period            statementPeriod
	Choices           []periodChoice
	ExportURL         template.URL
	Totals            StatementTotals
	Orders            []StatementOrder
	HiddenOrders      int
	Payouts           []StatementPayout
	CommissionPercent int
	Generated         string
}

func statementFuncs() template.FuncMap {
	return template.FuncMap{"money": money, "naira2": formatNaira2}
}

// VendorStatements shows a vendor their statement for a period.
func (h *Handlers) VendorStatements(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	period, problem := parseStatementPeriod(r.URL.Query(), time.Now())
	st, err := h.loadVendorStatement(vendorID, period)
	if err == errStatementTooLarge {
		appErrors.Render(w, "This period has too many orders to show. Please choose a shorter period.", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("vendor statement: load failed: %v", err)
		appErrors.Render(w, "Could not load your statement", http.StatusInternalServerError)
		return
	}

	page := VendorStatementPage{
		VendorName:        vendorName,
		Notice:            problem,
		Period:            period,
		Choices:           periodChoices(period.Preset),
		ExportURL:         period.exportURL("/vendor/statements/export"),
		Totals:            st.Totals,
		Orders:            st.Orders,
		Payouts:           st.Payouts,
		CommissionPercent: commissionPercent(),
		Generated:         time.Now().In(watZone).Format("2 Jan 2006, 3:04 PM"),
	}
	if len(page.Orders) > statementDisplayCap {
		page.HiddenOrders = len(page.Orders) - statementDisplayCap
		page.Orders = page.Orders[:statementDisplayCap]
	}

	tmpl, err := render.Page("vendor-statements.html", statementFuncs())
	if err != nil {
		log.Printf("vendor statement: template failed: %v", err)
		appErrors.Render(w, "Could not display your statement", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor statement: execute failed: %v", err)
	}
}

// VendorStatementExport sends the statement as a spreadsheet-friendly CSV file.
func (h *Handlers) VendorStatementExport(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	period, _ := parseStatementPeriod(r.URL.Query(), time.Now())
	st, err := h.loadVendorStatement(vendorID, period)
	if err == errStatementTooLarge {
		appErrors.Render(w, "This period has too many orders to export. Please choose a shorter period.", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("vendor statement export: load failed: %v", err)
		appErrors.Render(w, "Could not export your statement", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+statementFilename("enie-statement", period)+`"`)
	w.Header().Set("Cache-Control", "no-store")

	cw := csv.NewWriter(w)
	cw.Write([]string{"ENIE vendor statement"})
	cw.Write([]string{"Shop", csvCell(vendorName)})
	cw.Write([]string{"Period", period.Label})
	cw.Write([]string{"Generated", time.Now().In(watZone).Format("2006-01-02 15:04") + " WAT"})
	cw.Write([]string{"Amounts are in naira (NGN)"})
	cw.Write(nil)
	cw.Write([]string{"Order", "Date (WAT)", "Method", "Status", "Food sales", "Coupon discount",
		"Loyalty points used", "Delivery fee", "Customer paid", "ENIE commission", "Your earnings"})
	for _, o := range st.Orders {
		cw.Write([]string{
			strconv.Itoa(o.ID), o.CreatedAt.Format("2006-01-02 15:04"), o.Method, o.Status,
			csvMoney(o.FoodKobo), csvMoney(o.CouponKobo), csvMoney(o.LoyaltyKobo), csvMoney(o.DeliveryKobo),
			csvMoney(o.CollectedKobo), csvMoney(o.CommissionKobo), csvMoney(o.NetKobo),
		})
	}
	t := st.Totals
	cw.Write([]string{"", "", "", "Totals", csvMoney(t.FoodKobo), csvMoney(t.CouponKobo), csvMoney(t.LoyaltyKobo),
		csvMoney(t.DeliveryKobo), csvMoney(t.CollectedKobo), csvMoney(t.CommissionKobo), csvMoney(t.NetKobo)})
	cw.Write(nil)
	cw.Write([]string{"Balance"})
	cw.Write([]string{"Opening balance", csvMoney(t.OpeningKobo)})
	cw.Write([]string{"Earnings in period", csvMoney(t.NetKobo)})
	cw.Write([]string{"Payouts received", csvMoney(t.PayoutsKobo)})
	cw.Write([]string{"Closing balance", csvMoney(t.ClosingKobo)})
	if len(st.Payouts) > 0 {
		cw.Write(nil)
		cw.Write([]string{"Payouts"})
		cw.Write([]string{"Date (WAT)", "Amount", "Note"})
		for _, po := range st.Payouts {
			cw.Write([]string{po.PaidAt.Format("2006-01-02"), csvMoney(po.AmountKobo), csvCell(po.Note)})
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		log.Printf("vendor statement export: write failed: %v", err)
	}
}
