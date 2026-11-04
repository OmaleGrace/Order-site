package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

// Group orders are for offices, parties and events. The customer asks a shop, the shop sends
// back one price, and the customer pays that price in full. Paying creates a normal order
// (scheduled for the event), so payouts, commission, cancellations and refunds all work as usual.

const (
	minGroupHeadcount = 5
	maxGroupHeadcount = 1000
	minGroupQuoteKobo = 500000    // N5,000
	maxGroupQuoteKobo = 500000000 // N5,000,000
)

func lagosLocation() *time.Location {
	loc, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		return time.FixedZone("WAT", 3600)
	}
	return loc
}

type GroupVendorOption struct {
	ID       int
	Name     string
	Selected bool
}

type GroupRequestForm struct {
	Vendors []GroupVendorOption
	MinDate string
	Error   string
	MinHead int
	MaxHead int
	Phone   string
}

// GroupOrderForm shows the request form.
func (h *Handlers) GroupOrderForm(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		errors.Render(w, "Please log in to request a group order.", http.StatusUnauthorized)
		return
	}
	selected, _ := strconv.Atoi(r.URL.Query().Get("vendor"))
	page := GroupRequestForm{
		MinDate: time.Now().In(lagosLocation()).Add(48 * time.Hour).Format("2006-01-02T15:04"),
		Error:   r.URL.Query().Get("err"),
		MinHead: minGroupHeadcount, MaxHead: maxGroupHeadcount,
	}
	_ = h.DB.QueryRow(`SELECT COALESCE(phone, '') FROM users WHERE id = $1`, userID).Scan(&page.Phone)
	rows, err := h.DB.Query(`SELECT id, name FROM vendors WHERE status = 'active' AND paid_until > NOW() ORDER BY name`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var v GroupVendorOption
			if rows.Scan(&v.ID, &v.Name) == nil {
				v.Selected = v.ID == selected
				page.Vendors = append(page.Vendors, v)
			}
		}
	}
	tmpl, err := render.Page("group-order.html", template.FuncMap{})
	if err != nil {
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, page)
}

// GroupOrderSubmit saves a request and tells the shop.
func (h *Handlers) GroupOrderSubmit(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		http.Redirect(w, r, "/login?next=/group-order", http.StatusSeeOther)
		return
	}
	back := func(msg string) {
		http.Redirect(w, r, "/group-order?err="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		back("Something went wrong. Please try again.")
		return
	}
	vendorID, _ := strconv.Atoi(r.FormValue("vendor_id"))
	var vendorName string
	var ownerID sql.NullInt64
	if err := h.DB.QueryRow(`SELECT name, owner_user_id FROM vendors WHERE id = $1 AND status = 'active' AND paid_until > NOW()`, vendorID).Scan(&vendorName, &ownerID); err != nil {
		back("Please choose a shop.")
		return
	}
	eventAt, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(r.FormValue("event_at")), lagosLocation())
	if err != nil || eventAt.Before(time.Now().Add(24*time.Hour)) || eventAt.After(time.Now().AddDate(0, 6, 0)) {
		back("Choose a date and time at least a day ahead, and within the next 6 months.")
		return
	}
	headcount, _ := strconv.Atoi(r.FormValue("headcount"))
	if headcount < minGroupHeadcount || headcount > maxGroupHeadcount {
		back(fmt.Sprintf("A group order is for %d to %d people.", minGroupHeadcount, maxGroupHeadcount))
		return
	}
	method := r.FormValue("delivery_method")
	if method != "delivery" {
		method = "pickup"
	}
	address := strings.TrimSpace(r.FormValue("address"))
	if len([]rune(address)) > 300 {
		back("The address is too long.")
		return
	}
	if method == "delivery" && len([]rune(address)) < 8 {
		back("Enter the full delivery address.")
		return
	}
	phone := normalizeNigerianPhone(strings.TrimSpace(r.FormValue("phone")))
	if phone == "" {
		back("Enter a valid Nigerian phone number the shop can call.")
		return
	}
	notes := strings.TrimSpace(r.FormValue("notes"))
	if n := len([]rune(notes)); n < 10 || n > 1000 {
		back("Tell the shop what you need (10 to 1,000 characters): dishes, dietary needs, budget.")
		return
	}
	var budget interface{}
	if b, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("budget"))); b > 0 && b <= 100000000 {
		budget = b * 100
	}

	var id int
	if err := h.DB.QueryRow(`
		INSERT INTO group_requests (user_id, vendor_id, event_at, headcount, delivery_method, address, phone, notes, budget_kobo)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		userID, vendorID, eventAt, headcount, method, address, phone, notes, budget).Scan(&id); err != nil {
		log.Printf("group order: insert failed: %v", err)
		back("We couldn't save your request. Please try again.")
		return
	}
	if ownerID.Valid {
		_ = createNotification(h.DB, int(ownerID.Int64), "order", "New group order request",
			fmt.Sprintf("A customer wants food for %d people on %s. Open Group orders to send a price.", headcount, eventAt.Format("2 Jan")), "/vendor/group-orders")
	}
	http.Redirect(w, r, "/group-orders?msg="+url.QueryEscape("Request sent to "+vendorName+". They will reply with a price."), http.StatusSeeOther)
}

type GroupRequestRow struct {
	ID          int
	Vendor      string
	Customer    string
	EventAt     string
	Headcount   int
	Method      string
	Address     string
	Phone       string
	Notes       string
	BudgetNaira int
	QuoteNaira  int
	VendorNote  string
	State       string // requested, quoted, awaiting_payment, paid, declined, cancelled
	OrderStatus string
	CanQuote    bool
}

type GroupListPage struct {
	Rows        []GroupRequestRow
	Notice      string
	WalletNaira int
}

func groupState(status string, orderID sql.NullInt64, orderStatus sql.NullString) (string, string) {
	if status == "accepted" && orderID.Valid {
		if orderStatus.String == "pending" {
			return "awaiting_payment", ""
		}
		return "paid", orderStatus.String
	}
	return status, ""
}

const groupSelect = `
	SELECT r.id, v.name, u.name, TO_CHAR(r.event_at AT TIME ZONE 'Africa/Lagos', 'Dy DD Mon YYYY, HH24:MI'),
	       r.headcount, r.delivery_method, r.address, r.phone, r.notes,
	       COALESCE(r.budget_kobo, 0), COALESCE(r.quote_kobo, 0), r.vendor_note, r.status, r.order_id, o.status
	FROM group_requests r
	JOIN vendors v ON v.id = r.vendor_id
	JOIN users u ON u.id = r.user_id
	LEFT JOIN orders o ON o.id = r.order_id `

func scanGroupRows(rows *sql.Rows) []GroupRequestRow {
	var out []GroupRequestRow
	for rows.Next() {
		var g GroupRequestRow
		var status string
		var orderID sql.NullInt64
		var orderStatus sql.NullString
		var budget, quote int
		if err := rows.Scan(&g.ID, &g.Vendor, &g.Customer, &g.EventAt, &g.Headcount, &g.Method, &g.Address, &g.Phone, &g.Notes,
			&budget, &quote, &g.VendorNote, &status, &orderID, &orderStatus); err != nil {
			continue
		}
		g.BudgetNaira, g.QuoteNaira = budget/100, quote/100
		g.State, g.OrderStatus = groupState(status, orderID, orderStatus)
		g.CanQuote = g.State == "requested" || g.State == "quoted"
		out = append(out, g)
	}
	return out
}

// GroupOrders lists the customer's own requests and lets them pay a quoted price.
func (h *Handlers) GroupOrders(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		errors.Render(w, "Please log in to see your group orders.", http.StatusUnauthorized)
		return
	}
	page := GroupListPage{Notice: r.URL.Query().Get("msg"), WalletNaira: walletBalance(h.DB, userID) / 100}
	rows, err := h.DB.Query(groupSelect+`WHERE r.user_id = $1 ORDER BY r.id DESC LIMIT 50`, userID)
	if err != nil {
		log.Printf("group orders: list failed: %v", err)
		errors.Render(w, "Could not load your group orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	page.Rows = scanGroupRows(rows)
	tmpl, err := render.Page("group-orders.html", template.FuncMap{})
	if err != nil {
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, page)
}

// prepareGroupOrder turns an accepted quote into a pending order (or finds the one already made).
// It returns the order id and the amount in kobo.
func (h *Handlers) prepareGroupOrder(requestID, userID int) (int, int, error) {
	tx, err := h.DB.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	var vendorID, quote int
	var status, method, address, phone string
	var eventAt time.Time
	var orderID sql.NullInt64
	var orderStatus sql.NullString
	err = tx.QueryRow(`
		SELECT r.vendor_id, r.status, COALESCE(r.quote_kobo, 0), r.event_at, r.delivery_method, r.address, r.phone, r.order_id, o.status
		FROM group_requests r LEFT JOIN orders o ON o.id = r.order_id
		WHERE r.id = $1 AND r.user_id = $2 FOR UPDATE OF r`, requestID, userID).Scan(&vendorID, &status, &quote, &eventAt, &method, &address, &phone, &orderID, &orderStatus)
	if err != nil {
		return 0, 0, err
	}
	if status == "accepted" && orderID.Valid {
		if orderStatus.String != "pending" {
			return 0, 0, fmt.Errorf("this group order is already paid")
		}
		return int(orderID.Int64), quote, nil
	}
	if status != "quoted" || quote < minGroupQuoteKobo {
		return 0, 0, fmt.Errorf("there is no price to pay yet")
	}
	if !eventAt.After(time.Now()) {
		return 0, 0, fmt.Errorf("the event date has passed")
	}
	var newOrder int
	if err := tx.QueryRow(`
		INSERT INTO orders (user_id, total_kobo, status, vendor_id, delivery_method, delivery_address, delivery_phone, commission_kobo, scheduled_for)
		VALUES ($1, $2, 'pending', $3, $4, $5, $6, $7, $8) RETURNING id`,
		userID, quote, vendorID, method, address, phone, commissionKoboFor(h.DB, vendorID, quote), eventAt).Scan(&newOrder); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(`UPDATE group_requests SET status = 'accepted', order_id = $2, updated_at = NOW() WHERE id = $1`, requestID, newOrder); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return newOrder, quote, nil
}

// GroupOrderPay pays a quoted price from the wallet or by card.
func (h *Handlers) GroupOrderPay(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		http.Redirect(w, r, "/login?next=/group-orders", http.StatusSeeOther)
		return
	}
	back := func(msg string) {
		http.Redirect(w, r, "/group-orders?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		back("Something went wrong. Please try again.")
		return
	}
	requestID, _ := strconv.Atoi(r.FormValue("id"))
	method := r.FormValue("method")

	// For the wallet, check the balance before making an order at all.
	if method == "wallet" {
		var quote int
		_ = h.DB.QueryRow(`SELECT COALESCE(quote_kobo, 0) FROM group_requests WHERE id = $1 AND user_id = $2`, requestID, userID).Scan(&quote)
		if quote > 0 && walletBalance(h.DB, userID) < quote {
			back("Your wallet balance is too low. Top up your wallet, or pay by card.")
			return
		}
	}
	orderID, amount, err := h.prepareGroupOrder(requestID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			back("We couldn't find that group order.")
		} else {
			back("Sorry, that can't be paid right now: " + err.Error() + ".")
		}
		return
	}

	if method == "wallet" {
		if err := h.payOrderWithWallet(orderID, userID); err != nil {
			if err == errWalletInsufficient {
				back("Your wallet balance is too low. Top up your wallet, or pay by card.")
				return
			}
			log.Printf("group order %d: wallet payment failed: %v", requestID, err)
			back("We couldn't take the payment from your wallet. Please try again.")
			return
		}
		back("Paid! Your group order is confirmed and the shop has been told.")
		return
	}

	callback := os.Getenv("PAYSTACK_CALLBACK_URL")
	var userEmail string
	if err := h.DB.QueryRow(`SELECT email FROM users WHERE id = $1`, userID).Scan(&userEmail); err != nil || callback == "" {
		back("Card payment isn't available right now.")
		return
	}
	reference := fmt.Sprintf("ORDER-%d-%d", orderID, time.Now().UnixNano())
	authURL, err := paystackInitialize(userEmail, amount, reference, callback)
	if err != nil {
		log.Printf("group order %d: paystack failed: %v", requestID, err)
		back("Could not connect to the payment service. Please try again.")
		return
	}
	if _, err := h.DB.Exec(`UPDATE orders SET payment_reference = $1 WHERE id = $2`, reference, orderID); err != nil {
		log.Printf("group order %d: save reference failed: %v", requestID, err)
		back("Could not start the payment. Please try again.")
		return
	}
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

// GroupOrderCancel lets the customer withdraw a request that has not been paid.
func (h *Handlers) GroupOrderCancel(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	requestID, _ := strconv.Atoi(r.FormValue("id"))
	tx, err := h.DB.Begin()
	if err != nil {
		http.Redirect(w, r, "/group-orders", http.StatusSeeOther)
		return
	}
	defer tx.Rollback()
	var orderID sql.NullInt64
	var status string
	if err := tx.QueryRow(`SELECT status, order_id FROM group_requests WHERE id = $1 AND user_id = $2 FOR UPDATE`, requestID, userID).Scan(&status, &orderID); err != nil {
		http.Redirect(w, r, "/group-orders", http.StatusSeeOther)
		return
	}
	msg := "Request withdrawn."
	switch {
	case status == "requested" || status == "quoted":
		_, _ = tx.Exec(`UPDATE group_requests SET status = 'cancelled', updated_at = NOW() WHERE id = $1`, requestID)
	case status == "accepted" && orderID.Valid:
		res, _ := tx.Exec(`UPDATE orders SET status = 'cancelled' WHERE id = $1 AND status = 'pending'`, orderID.Int64)
		if n, _ := res.RowsAffected(); n == 1 {
			_, _ = tx.Exec(`UPDATE group_requests SET status = 'cancelled', updated_at = NOW() WHERE id = $1`, requestID)
		} else {
			msg = "This group order is already paid. To cancel it, contact the shop or support."
		}
	default:
		msg = "That request can't be withdrawn."
	}
	if err := tx.Commit(); err != nil {
		msg = "Could not withdraw the request. Please try again."
	}
	http.Redirect(w, r, "/group-orders?msg="+url.QueryEscape(msg), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// Vendor side
// ---------------------------------------------------------------------------

func (h *Handlers) VendorGroupOrders(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	page := GroupListPage{Notice: r.URL.Query().Get("msg")}
	rows, err := h.DB.Query(groupSelect+`WHERE r.vendor_id = $1 ORDER BY r.id DESC LIMIT 100`, vendorID)
	if err != nil {
		log.Printf("vendor group orders: list failed: %v", err)
		errors.Render(w, "Could not load group orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	page.Rows = scanGroupRows(rows)
	tmpl, err := render.Page("vendor-group-orders.html", template.FuncMap{})
	if err != nil {
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, page)
}

// VendorGroupOrderQuote sets the one price the customer will pay.
func (h *Handlers) VendorGroupOrderQuote(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	back := func(msg string) {
		http.Redirect(w, r, "/vendor/group-orders?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		back("Something went wrong.")
		return
	}
	id, _ := strconv.Atoi(r.FormValue("id"))
	if r.FormValue("action") == "decline" {
		res, err := h.DB.Exec(`UPDATE group_requests SET status = 'declined', vendor_note = $3, updated_at = NOW()
			WHERE id = $1 AND vendor_id = $2 AND status IN ('requested', 'quoted')`, id, vendorID, strings.TrimSpace(r.FormValue("note")))
		if err != nil {
			back("Could not decline the request.")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			back("That request can't be declined now.")
			return
		}
		h.notifyGroupCustomer(id, "Group order declined", "The shop can't take your group order. You can request another shop.")
		back("Request declined.")
		return
	}
	naira, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("quote")))
	quote := naira * 100
	if quote < minGroupQuoteKobo || quote > maxGroupQuoteKobo {
		back(fmt.Sprintf("Enter a price between ₦%d and ₦%d.", minGroupQuoteKobo/100, maxGroupQuoteKobo/100))
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))
	if len([]rune(note)) > 500 {
		note = string([]rune(note)[:500])
	}
	res, err := h.DB.Exec(`
		UPDATE group_requests SET status = 'quoted', quote_kobo = $3, vendor_note = $4, updated_at = NOW()
		WHERE id = $1 AND vendor_id = $2 AND status IN ('requested', 'quoted') AND order_id IS NULL`, id, vendorID, quote, note)
	if err != nil {
		log.Printf("vendor group quote failed: %v", err)
		back("Could not save the price.")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		back("That request can't be priced now.")
		return
	}
	h.notifyGroupCustomer(id, "Your group order has a price", fmt.Sprintf("The shop quoted ₦%d. Open Group orders to pay and confirm.", naira))
	back("Price sent to the customer.")
}

func (h *Handlers) notifyGroupCustomer(requestID int, title, message string) {
	var userID int
	if err := h.DB.QueryRow(`SELECT user_id FROM group_requests WHERE id = $1`, requestID).Scan(&userID); err == nil {
		_ = createNotification(h.DB, userID, "order", title, message, "/group-orders")
	}
}
