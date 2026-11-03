package handlers

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	htmltemplate "html/template"
	"os"
	"strings"
	"time"

	"Order-site/brand"
	"Order-site/email"
)

type orderEmailItem struct {
	Name      string
	Quantity  int
	PriceKobo int
	ImageURL  string
}

type orderEmailDetails struct {
	Reference  string
	Customer   string
	Vendor     string
	Email      string
	Status     string
	TotalKobo  int
	CreatedAt  time.Time
	Items      []orderEmailItem
	Brand      string
	StatusText string
	TrackURL   string // set for guest orders: the link that replaces "sign in and open My Orders"
	IsGuest    bool
}

// publicOrderReference gives customers a stable reference that does not expose
// the database's sequential order count. The internal numeric ID remains private.
//
// The hash seed below still says "enie-order" on purpose: references are computed rather
// than stored, so keeping the seed means every reference already sent to a customer keeps
// the same code after the rebrand (only the readable prefix changed from ENIE- to LAIKI-).
func publicOrderReference(id int, createdAt time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("enie-order:%d:%s", id, createdAt.UTC().Format(time.RFC3339Nano))))
	return "LAIKI-" + strings.ToUpper(fmt.Sprintf("%x", sum[:])[:10])
}

func emailImageURL(imageURL string) string {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return ""
	}
	if strings.HasPrefix(imageURL, "https://") || strings.HasPrefix(imageURL, "http://") {
		return imageURL
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
	}
	if base == "" {
		return ""
	}
	if !strings.HasPrefix(imageURL, "/") {
		imageURL = "/" + imageURL
	}
	return base + imageURL
}

func loadOrderEmailDetails(db *sql.DB, orderID int) (orderEmailDetails, error) {
	var d orderEmailDetails
	var scannedID int
	var guestEmail string
	err := db.QueryRow(`
		SELECT o.id, o.total_kobo, o.status, o.created_at,
		       COALESCE(NULLIF(o.guest_name, ''), u.name, 'Customer'),
		       COALESCE(NULLIF(o.guest_email, ''), u.email),
		       COALESCE(v.name, $2), o.guest_email
		FROM orders o
		JOIN users u ON u.id = o.user_id
		LEFT JOIN vendors v ON v.id = o.vendor_id
		WHERE o.id = $1
	`, orderID, brand.Name()).Scan(&scannedID, &d.TotalKobo, &d.Status, &d.CreatedAt, &d.Customer, &d.Email, &d.Vendor, &guestEmail)
	if err != nil {
		return d, err
	}
	if guestEmail != "" {
		d.IsGuest = true
		d.TrackURL = trackingURL(orderID)
	}
	d.Reference = publicOrderReference(orderID, d.CreatedAt)
	d.Brand = brand.Name()
	rows, err := db.Query(`
		SELECT m.name, oi.quantity, oi.price_kobo, COALESCE(m.image_url, '')
		FROM order_items oi
		JOIN menu_items m ON m.id = oi.menu_item_id
		WHERE oi.order_id = $1
		ORDER BY oi.id
	`, orderID)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var item orderEmailItem
		if err := rows.Scan(&item.Name, &item.Quantity, &item.PriceKobo, &item.ImageURL); err != nil {
			return d, err
		}
		item.ImageURL = emailImageURL(item.ImageURL)
		d.Items = append(d.Items, item)
	}
	return d, rows.Err()
}

const orderEmailTemplate = `<!doctype html><html><body style="margin:0;background:#f5f5f0;font-family:Arial,sans-serif;color:#202820"><div style="max-width:620px;margin:24px auto;background:#fff;border-radius:14px;overflow:hidden"><div style="background:#174c35;color:#fff;padding:24px"><h1 style="margin:0">{{.Brand}}</h1><p style="margin:8px 0 0">{{.StatusText}}</p></div><div style="padding:24px"><p>Hello {{.Customer}},</p><p>Your order reference is <strong>{{.Reference}}</strong>.</p><p>Shop: <strong>{{.Vendor}}</strong></p><p>Current status: <strong>{{.Status}}</strong></p><h2 style="font-size:18px">Order summary</h2>{{range .Items}}<div style="display:flex;border-bottom:1px solid #e6e6e0;padding:12px 0;align-items:center">{{if .ImageURL}}<img src="{{.ImageURL}}" alt="{{.Name}}" width="72" height="72" style="object-fit:cover;border-radius:8px;margin-right:12px">{{end}}<div style="flex:1"><strong>{{.Name}}</strong><br>Quantity: {{.Quantity}}<br>₦{{naira .PriceKobo}}</div></div>{{end}}<p style="font-size:18px"><strong>Total: ₦{{naira .TotalKobo}}</strong></p>{{if .TrackURL}}<p style="margin:20px 0"><a href="{{.TrackURL}}" style="background:#174a32;color:#fff;text-decoration:none;padding:12px 20px;border-radius:8px;display:inline-block;font-weight:bold">Track your order</a></p><p style="font-size:13px;color:#666">Keep this email. The button above is the only way to check your order, so please don't forward it to anyone you don't want to see your order.</p>{{else if .IsGuest}}<p>Keep your order reference. If you need an update, contact the shop or {{.Brand}} support with it.</p>{{else}}<p>You can track your order by signing in to {{.Brand}} and opening My Orders.</p>{{end}}<p style="color:#666;font-size:12px">Please keep your order reference for support enquiries.</p></div></div></body></html>`

func sendOrderEmail(db *sql.DB, orderID int, statusText string) error {
	d, err := loadOrderEmailDetails(db, orderID)
	if err != nil {
		return fmt.Errorf("load order email details: %w", err)
	}
	if strings.TrimSpace(d.Email) == "" {
		return fmt.Errorf("order %s has no customer email", d.Reference)
	}
	d.StatusText = statusText
	tmpl, err := htmltemplate.New("order-email").Funcs(htmltemplate.FuncMap{"naira": func(kobo int) int { return kobo / 100 }}).Parse(orderEmailTemplate)
	if err != nil {
		return err
	}
	var body strings.Builder
	if err := tmpl.Execute(&body, d); err != nil {
		return err
	}
	subject := fmt.Sprintf("%s — %s", statusText, d.Reference)
	return email.Send(d.Email, subject, body.String())
}

func sendOrderPlacedEmail(db *sql.DB, orderID int) error {
	return sendOrderEmail(db, orderID, "Your order is confirmed")
}

func sendOrderStatusEmail(db *sql.DB, orderID int, status string) error {
	return sendOrderEmail(db, orderID, "Order update: "+strings.Title(strings.ReplaceAll(status, "_", " ")))
}
