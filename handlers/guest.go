package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

// errTooManyGuests is returned when one address starts too many guest carts in an hour.
var errTooManyGuests = fmt.Errorf("too many guest carts started from this address")

// guestStartLimiter stops a script from filling the database with guest rows.
var guestStartLimiter = middleware.NewLimiter(40, time.Hour)

// cartUser works out whose cart a request belongs to. A logged-in customer wins; otherwise
// a guest cart session is used. With create set, a visitor who has neither gets a fresh
// guest cart session (this is what happens on their first "Add to cart").
//
// It returns sql.ErrNoRows when there is no cart owner and create is false. The second
// result says whether the owner is a guest.
func (h *Handlers) cartUser(w http.ResponseWriter, r *http.Request, create bool) (int, bool, error) {
	if c, err := r.Cookie("session_token"); err == nil && c.Value != "" {
		id, lerr := middleware.GetUserID(h.DB, c.Value)
		if lerr == nil {
			return id, false, nil
		}
		if lerr != sql.ErrNoRows {
			return 0, false, lerr
		}
	}

	if c, err := r.Cookie(middleware.GuestCookieName); err == nil && c.Value != "" {
		id, lerr := middleware.GetGuestUserID(h.DB, c.Value)
		if lerr == nil {
			return id, true, nil
		}
		if lerr != sql.ErrNoRows {
			return 0, false, lerr
		}
	}

	if !create {
		return 0, false, sql.ErrNoRows
	}
	if !guestStartLimiter.Hit(middleware.ClientIP(r)) {
		return 0, false, errTooManyGuests
	}

	id, err := middleware.CreateGuestUser(h.DB)
	if err != nil {
		return 0, false, err
	}
	token, err := middleware.CreateSessionWithDuration(h.DB, id, middleware.GuestSessionDuration)
	if err != nil {
		return 0, false, err
	}
	middleware.SetGuestCookie(w, r, token)
	return id, true, nil
}

// ---------------------------------------------------------------------------
// Order tracking links
// ---------------------------------------------------------------------------

// trackingSecret is the key that signs tracking links. ORDER_TRACKING_SECRET is preferred;
// the payment secret is used when it is not set, so links work without extra setup.
func trackingSecret() []byte {
	for _, name := range []string{"ORDER_TRACKING_SECRET", "PAYSTACK_SECRET_KEY"} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return []byte(v)
		}
	}
	return nil
}

// trackingToken is a signature only the server can produce for an order. Nothing is
// stored: the link is checked by recomputing the signature, so it cannot be guessed
// from the order number and cannot be forged.
func trackingToken(orderID int) string {
	secret := trackingSecret()
	if len(secret) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, secret)
	fmt.Fprintf(mac, "order-tracking:%d", orderID)
	return hex.EncodeToString(mac.Sum(nil))
}

func validTrackingToken(orderID int, token string) bool {
	want := trackingToken(orderID)
	return want != "" && hmac.Equal([]byte(want), []byte(token))
}

// trackingPath is the site-relative tracking address for an order.
func trackingPath(orderID int) string {
	return fmt.Sprintf("/track/%d/%s", orderID, trackingToken(orderID))
}

// trackingURL is the full address for emails. It is empty when the public base URL is
// not configured, because a relative link is useless in an email.
func trackingURL(orderID int) string {
	base := brand.SiteURL()
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/")
	}
	if base == "" || trackingToken(orderID) == "" {
		return ""
	}
	return base + trackingPath(orderID)
}

// afterPaymentPath is where a customer lands once payment is confirmed. Guests have no
// "My Orders" page, so they go straight to their tracking page.
func (h *Handlers) afterPaymentPath(orderID int) string {
	var guestEmail string
	if err := h.DB.QueryRow(`SELECT guest_email FROM orders WHERE id = $1`, orderID).Scan(&guestEmail); err == nil && guestEmail != "" {
		if trackingToken(orderID) != "" {
			return trackingPath(orderID)
		}
	}
	return "/order-success"
}

// ---------------------------------------------------------------------------
// Tracking page
// ---------------------------------------------------------------------------

type TrackItem struct {
	Name     string
	Quantity int
	LineKobo int
	Options  []string
}

type TrackStep struct {
	Label string
	State string // done, current or todo
}

type TrackPage struct {
	Reference       string
	Customer        string
	Vendor          string
	VendorAddress   string
	VendorPhone     string
	Status          string
	StatusLabel     string
	StatusNote      string
	Cancelled       bool
	AwaitingPayment bool
	Finished        bool
	Steps           []TrackStep
	Items           []TrackItem
	IsDelivery      bool
	Address         string
	DeliveryPhone   string
	RiderName       string
	RiderPhone      string
	FoodKobo        int
	DeliveryKobo    int
	DiscountKobo    int
	TotalKobo       int
	PlacedAt        string
	ScheduledFor    string
}

// TrackOrder shows one guest order's progress. The link is the only credential, so the
// page is never cached or indexed and does not leak the address to other sites.
func (h *Handlers) TrackOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	orderID, err := strconv.Atoi(r.PathValue("orderID"))
	if err != nil || orderID <= 0 || !validTrackingToken(orderID, r.PathValue("token")) {
		errors.Render(w, "We couldn't find that order. Please use the tracking link from your confirmation email.", http.StatusNotFound)
		return
	}

	var page TrackPage
	var createdAt time.Time
	var scheduled sql.NullTime
	var method, deliveryAddress, lga, state string
	err = h.DB.QueryRow(`
		SELECT o.guest_name, o.status, o.created_at, o.scheduled_for,
		       o.delivery_method, o.delivery_address, o.delivery_lga, o.delivery_state, o.delivery_phone,
		       o.delivery_fee_kobo, o.total_kobo,
		       COALESCE(v.name, ''), COALESCE(v.address, ''), COALESCE(v.phone, ''),
		       COALESCE(r.name, ''), COALESCE(r.phone, '')
		FROM orders o
		LEFT JOIN vendors v ON v.id = o.vendor_id
		LEFT JOIN riders r ON r.id = o.rider_id
		WHERE o.id = $1 AND o.guest_email <> ''
	`, orderID).Scan(&page.Customer, &page.Status, &createdAt, &scheduled,
		&method, &deliveryAddress, &lga, &state, &page.DeliveryPhone,
		&page.DeliveryKobo, &page.TotalKobo,
		&page.Vendor, &page.VendorAddress, &page.VendorPhone,
		&page.RiderName, &page.RiderPhone)
	if err == sql.ErrNoRows {
		errors.Render(w, "We couldn't find that order. Please use the tracking link from your confirmation email.", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("track order %d: lookup failed: %v", orderID, err)
		errors.Render(w, "We couldn't load your order right now. Please try again in a moment.", http.StatusInternalServerError)
		return
	}

	page.Reference = publicOrderReference(orderID, createdAt)
	page.PlacedAt = createdAt.Format("2 Jan 2006, 15:04")
	if scheduled.Valid {
		loc, locErr := time.LoadLocation("Africa/Lagos")
		if locErr != nil {
			loc = time.FixedZone("WAT", 3600)
		}
		page.ScheduledFor = scheduled.Time.In(loc).Format("Mon 2 Jan 2006, 15:04")
	}
	page.IsDelivery = method == "delivery"
	if page.IsDelivery {
		page.Address = strings.TrimSpace(strings.Join([]string{deliveryAddress, lga, state}, ", "))
	}

	// Items, with the customizations that were chosen for each one.
	rows, err := h.DB.Query(`
		SELECT oi.id, m.name, oi.quantity, oi.price_kobo
		FROM order_items oi
		JOIN menu_items m ON m.id = oi.menu_item_id
		WHERE oi.order_id = $1
		ORDER BY oi.id
	`, orderID)
	if err != nil {
		log.Printf("track order %d: items failed: %v", orderID, err)
		errors.Render(w, "We couldn't load your order right now. Please try again in a moment.", http.StatusInternalServerError)
		return
	}
	var itemIDs []int
	for rows.Next() {
		var id int
		var item TrackItem
		var price int
		if err := rows.Scan(&id, &item.Name, &item.Quantity, &price); err != nil {
			rows.Close()
			log.Printf("track order %d: item scan failed: %v", orderID, err)
			errors.Render(w, "We couldn't load your order right now. Please try again in a moment.", http.StatusInternalServerError)
			return
		}
		item.LineKobo = price * item.Quantity
		page.FoodKobo += item.LineKobo
		itemIDs = append(itemIDs, id)
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		log.Printf("track order %d: items failed: %v", orderID, err)
		errors.Render(w, "We couldn't load your order right now. Please try again in a moment.", http.StatusInternalServerError)
		return
	}
	rows.Close()

	for i, itemID := range itemIDs {
		optionRows, err := h.DB.Query(`SELECT name FROM order_item_options WHERE order_item_id = $1 ORDER BY id`, itemID)
		if err != nil {
			continue // the order is still shown without its customization notes
		}
		for optionRows.Next() {
			var name string
			if optionRows.Scan(&name) == nil {
				page.Items[i].Options = append(page.Items[i].Options, name)
			}
		}
		optionRows.Close()
	}

	// Whatever the food and delivery came to, less what was actually charged, was a discount.
	if d := page.FoodKobo + page.DeliveryKobo - page.TotalKobo; d > 0 {
		page.DiscountKobo = d
	}

	describeTrackStatus(&page)

	tmpl, err := render.Page("track-order.html", template.FuncMap{"naira": naira})
	if err != nil {
		log.Printf("track order: template failed: %v", err)
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("track order: execute failed: %v", err)
	}
}

// describeTrackStatus turns the stored status into plain words and a progress bar.
func describeTrackStatus(p *TrackPage) {
	steps := []string{"Order received", "Being prepared", "Ready", "Completed"}
	if p.IsDelivery {
		steps[2] = "Ready for delivery"
		steps[3] = "Delivered"
	} else {
		steps[2] = "Ready for pickup"
		steps[3] = "Collected"
	}

	current := -1
	switch p.Status {
	case "pending":
		p.AwaitingPayment = true
		p.StatusLabel = "Awaiting payment"
		p.StatusNote = "We haven't received your payment yet. If you have just paid, refresh this page in a minute."
	case "scheduled":
		current = 0
		p.StatusLabel = "Scheduled"
		p.StatusNote = "Your payment is confirmed. The shop will start preparing your order at the scheduled time."
	case "paid":
		current = 0
		p.StatusLabel = "Order received"
		p.StatusNote = "Your payment is confirmed and the shop has your order."
	case "preparing":
		current = 1
		p.StatusLabel = "Being prepared"
		p.StatusNote = "The shop is preparing your food."
	case "ready":
		current = 2
		if p.IsDelivery {
			p.StatusLabel = "Ready for delivery"
			p.StatusNote = "Your order is ready and is on its way to being delivered."
		} else {
			p.StatusLabel = "Ready for pickup"
			p.StatusNote = "Your order is ready. You can collect it from the shop."
		}
	case "completed":
		current = 3
		p.Finished = true
		if p.IsDelivery {
			p.StatusLabel = "Delivered"
			p.StatusNote = "Your order has been delivered. Enjoy your meal!"
		} else {
			p.StatusLabel = "Collected"
			p.StatusNote = "Your order has been collected. Enjoy your meal!"
		}
	case "cancelled":
		p.Cancelled = true
		p.Finished = true
		p.StatusLabel = "Cancelled"
		p.StatusNote = "This order was cancelled. If you were charged, please contact the shop or our support team."
	default:
		p.StatusLabel = strings.Title(strings.ReplaceAll(p.Status, "_", " "))
	}

	for i, label := range steps {
		state := "todo"
		switch {
		case current > i:
			state = "done"
		case current == i && p.Finished:
			state = "done"
		case current == i:
			state = "current"
		}
		p.Steps = append(p.Steps, TrackStep{Label: label, State: state})
	}
}
