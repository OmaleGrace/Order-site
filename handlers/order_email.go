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
}

// publicOrderReference gives customers a stable reference that does not expose
// the database's sequential order count. The internal numeric ID remains private.
func publicOrderReference(id int, createdAt time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("enie-order:%d:%s", id, createdAt.UTC().Format(time.RFC3339Nano))))
	return "ENIE-" + strings.ToUpper(fmt.Sprintf("%x", sum[:])[:10])
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
	err := db.QueryRow(`
		SELECT o.id, o.total_kobo, o.status, o.created_at,
		       COALESCE(u.name, 'Customer'), u.email,
		       COALESCE(v.name, $2)
		FROM orders o
		JOIN users u ON u.id = o.user_id
		LEFT JOIN vendors v ON v.id = o.vendor_id
		WHERE o.id = $1
	`, orderID, brand.Name()).Scan(&scannedID, &d.TotalKobo, &d.Status, &d.CreatedAt, &d.Customer, &d.Email, &d.Vendor)
	if err != nil {
		return d, err
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

const orderEmailTemplate = `<!doctype html><html><body style="margin:0;background:#f5f5f0;font-family:Arial,sans-serif;color:#202820"><div style="max-width:620px;margin:24px auto;background:#fff;border-radius:14px;overflow:hidden"><div style="background:#174c35;color:#fff;padding:24px"><h1 style="margin:0">{{.Brand}}</h1><p style="margin:8px 0 0">{{.StatusText}}</p></div><div style="padding:24px"><p>Hello {{.Customer}},</p><p>Your order reference is <strong>{{.Reference}}</strong>.</p><p>Shop: <strong>{{.Vendor}}</strong></p><p>Current status: <strong>{{.Status}}</strong></p><h2 style="font-size:18px">Order summary</h2>{{range .Items}}<div style="display:flex;border-bottom:1px solid #e6e6e0;padding:12px 0;align-items:center">{{if .ImageURL}}<img src="{{.ImageURL}}" alt="{{.Name}}" width="72" height="72" style="object-fit:cover;border-radius:8px;margin-right:12px">{{end}}<div style="flex:1"><strong>{{.Name}}</strong><br>Quantity: {{.Quantity}}<br>₦{{naira .PriceKobo}}</div></div>{{end}}<p style="font-size:18px"><strong>Total: ₦{{naira .TotalKobo}}</strong></p><p>You can track your order by signing in to {{.Brand}} and opening My Orders.</p><p style="color:#666;font-size:12px">Please keep your order reference for support enquiries.</p></div></div></body></html>`

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
