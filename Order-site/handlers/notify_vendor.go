package handlers

import (
	"database/sql"
	"fmt"
	"html"
	"log"
	"os"
	"strings"

	"Order-site/brand"
	"Order-site/email"
	"Order-site/safe"
	"Order-site/sms"
)

// notifyVendorNewOrder tells the shop owner a paid order is waiting: an in-app notification, an
// email, and (when Termii is set up) a text message. Slow vendors cause cancelled orders, so the
// alert goes out the moment payment is confirmed. Failures are logged and never block the customer.
func notifyVendorNewOrder(db *sql.DB, orderID int) {
	defer safe.Recover("vendor new-order alert")
	var ownerID int
	var shop, ownerEmail, shopPhone, method, address, customer string
	var total int
	err := db.QueryRow(`
		SELECT v.owner_user_id, v.name, COALESCE(u.email, ''), COALESCE(v.phone, ''),
		       o.total_kobo, COALESCE(o.delivery_method, 'pickup'), COALESCE(o.delivery_address, ''), COALESCE(c.name, 'A customer')
		FROM orders o
		JOIN vendors v ON v.id = o.vendor_id
		JOIN users u ON u.id = v.owner_user_id
		JOIN users c ON c.id = o.user_id
		WHERE o.id = $1
	`, orderID).Scan(&ownerID, &shop, &ownerEmail, &shopPhone, &total, &method, &address, &customer)
	if err != nil {
		log.Printf("vendor order alert: order %d lookup failed: %v", orderID, err)
		return
	}

	rows, err := db.Query(`
		SELECT m.name, oi.quantity FROM order_items oi JOIN menu_items m ON m.id = oi.menu_item_id
		WHERE oi.order_id = $1 ORDER BY oi.id`, orderID)
	var lines []string
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var qty int
			if rows.Scan(&name, &qty) == nil {
				lines = append(lines, fmt.Sprintf("%d × %s", qty, name))
			}
		}
	}

	how := "Pickup"
	if method == "delivery" {
		how = "Delivery"
	}
	title := "New order to prepare"
	summary := fmt.Sprintf("Order #%d, ₦%s. %s. %s", orderID, money(total), how, strings.Join(lines, ", "))
	if len([]rune(summary)) > 240 {
		summary = string([]rune(summary)[:237]) + "..."
	}
	if err := createNotification(db, ownerID, "vendor_order", title, summary, "/vendor/orders"); err != nil {
		log.Printf("vendor order alert: in-app notification failed for order %d: %v", orderID, err)
	}

	if ownerEmail != "" {
		var b strings.Builder
		b.WriteString(`<div style="font-family:Arial,sans-serif;max-width:560px;margin:0 auto;color:#202820">`)
		b.WriteString(`<h2 style="color:#174a32">New order for ` + html.EscapeString(shop) + `</h2>`)
		b.WriteString(fmt.Sprintf(`<p><strong>Order #%d</strong> from %s has been paid. Please start preparing it.</p>`, orderID, html.EscapeString(customer)))
		b.WriteString(`<ul>`)
		for _, l := range lines {
			b.WriteString(`<li>` + html.EscapeString(l) + `</li>`)
		}
		b.WriteString(`</ul>`)
		b.WriteString(fmt.Sprintf(`<p>Total: <strong>₦%s</strong> · %s`, money(total), how))
		if method == "delivery" && address != "" {
			b.WriteString(`<br>Deliver to: ` + html.EscapeString(address))
		}
		b.WriteString(`</p>`)
		if base := brand.SiteURL(); base != "" {
			b.WriteString(`<p><a href="` + html.EscapeString(base) + `/vendor/orders" style="background:#174a32;color:#fff;padding:12px 18px;border-radius:8px;text-decoration:none">Open my orders</a></p>`)
		}
		b.WriteString(`<p style="color:#667;font-size:12px">Mark the order as preparing, ready and completed so your customer can follow it, and your payout is released.</p></div>`)
		if err := email.Send(ownerEmail, fmt.Sprintf("New order #%d – ₦%s", orderID, money(total)), b.String()); err != nil {
			log.Printf("vendor order alert: email failed for order %d: %v", orderID, err)
		}
	}

	if shopPhone != "" && os.Getenv("TERMII_API_KEY") != "" && !strings.EqualFold(os.Getenv("VENDOR_ORDER_SMS"), "off") {
		text := fmt.Sprintf("%s: new paid order #%d, N%s (%s). Open %s/vendor/orders to start.", brand.Name(), orderID, money(total), how, strings.TrimPrefix(brand.SiteURL(), "https://"))
		if err := sms.SendText(shopPhone, text); err != nil {
			log.Printf("vendor order alert: SMS failed for order %d: %v", orderID, err)
		}
	}
}
