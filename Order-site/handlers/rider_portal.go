package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

func newRiderToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}
func riderTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func riderAccessURL(r *http.Request, token string) string {
	base := brand.SiteURL()
	if base == "" {
		scheme := "http"
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return html.EscapeString(strings.TrimRight(base, "/") + "/rider/" + token)
}

// VendorAssignRider only permits assigning a rider to a delivery order belonging to the signed-in vendor.
func (h *Handlers) VendorAssignRider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", 405)
		return
	}
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	orderID, _ := strconv.Atoi(r.FormValue("order_id"))
	riderID, _ := strconv.Atoi(r.FormValue("rider_id"))
	if orderID < 1 {
		http.Redirect(w, r, "/vendor/orders", 303)
		return
	}
	var token, hash string
	var err error
	if riderID > 0 {
		token, hash, err = newRiderToken()
		if err == nil {
			var res sql.Result
			res, err = h.DB.Exec(`UPDATE orders SET rider_id=$1,delivery_status='assigned',rider_access_token_hash=$2,rider_sharing_location=FALSE,rider_latitude=NULL,rider_longitude=NULL,rider_location_accuracy=NULL,rider_location_updated_at=NULL,rider_status_updated_at=NOW() WHERE id=$3 AND vendor_id=$4 AND delivery_method='delivery' AND status NOT IN ('pending','cancelled','completed') AND EXISTS(SELECT 1 FROM riders WHERE id=$1 AND active=TRUE)`, riderID, hash, orderID, vendorID)
			if err == nil {
				if n, _ := res.RowsAffected(); n == 0 {
					err = fmt.Errorf("order or active rider not found")
				}
			}
		}
	} else {
		_, err = h.DB.Exec(`UPDATE orders SET rider_id=NULL,delivery_status='unassigned',rider_access_token_hash=NULL,rider_sharing_location=FALSE,rider_latitude=NULL,rider_longitude=NULL,rider_location_accuracy=NULL,rider_location_updated_at=NULL WHERE id=$1 AND vendor_id=$2 AND delivery_method='delivery'`, orderID, vendorID)
	}
	if err != nil {
		log.Printf("vendor rider assignment failed: %v", err)
		errors.Render(w, "Could not assign rider. Check that the rider is active and the order is a delivery.", 500)
		return
	}
	if token != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		link := riderAccessURL(r, token)
		fmt.Fprintf(w, `<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Rider assigned</title></head><body style="font:16px system-ui;max-width:620px;margin:40px auto;padding:20px"><h1>Rider assigned</h1><p>Copy this private link and send it directly to the rider. Anyone with the link can update this delivery, so do not post it publicly.</p><p><a href="%s">Open rider tracking page</a></p><p><code>%s</code></p><p><a href="/vendor/orders">Return to vendor orders</a></p></body></html>`, link, link)
		return
	}
	http.Redirect(w, r, "/vendor/orders", 303)
}

type RiderPortalPage struct {
	RiderName      string
	OrderID        int
	DeliveryStatus string
	Sharing        bool
}

func (h *Handlers) RiderPortal(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if len(token) != 64 {
		errors.Render(w, "Invalid rider link", 404)
		return
	}
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	var p RiderPortalPage
	err := h.DB.QueryRow(`SELECT r.name,o.id,COALESCE(o.delivery_status,'assigned'),o.rider_sharing_location FROM orders o JOIN riders r ON r.id=o.rider_id WHERE o.rider_access_token_hash=$1 AND r.active=TRUE AND o.status NOT IN ('completed','cancelled')`, riderTokenHash(token)).Scan(&p.RiderName, &p.OrderID, &p.DeliveryStatus, &p.Sharing)
	if err != nil {
		errors.Render(w, "This rider link is invalid, expired or no longer active.", 404)
		return
	}
	t, err := render.Page("rider-portal.html")
	if err != nil {
		errors.Render(w, "Could not load rider page", 500)
		return
	}
	if err = t.Execute(w, p); err != nil {
		log.Printf("rider portal template: %v", err)
	}
}
func (h *Handlers) RiderUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	token := r.PathValue("token")
	status := r.FormValue("status")
	allowed := map[string]bool{"picked_up": true, "on_the_way": true, "arrived": true, "delivered": true}
	if !allowed[status] {
		http.Error(w, "Invalid status", 400)
		return
	}
	res, err := h.DB.Exec(`UPDATE orders SET delivery_status=$1,rider_status_updated_at=NOW(),status=CASE WHEN $1='delivered' THEN 'completed' ELSE status END,completed_at=CASE WHEN $1='delivered' THEN COALESCE(completed_at,NOW()) ELSE completed_at END WHERE rider_access_token_hash=$2 AND rider_id IS NOT NULL AND status NOT IN ('cancelled','completed') AND EXISTS(SELECT 1 FROM riders rr WHERE rr.id=orders.rider_id AND rr.active=TRUE)`, status, riderTokenHash(token))
	if err != nil {
		http.Error(w, "Could not update delivery", 500)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "Tracking link is no longer active", 404)
		return
	}
	var customerID int
	if err := h.DB.QueryRow(`SELECT user_id FROM orders WHERE rider_access_token_hash=$1`, riderTokenHash(token)).Scan(&customerID); err == nil {
		label := strings.ReplaceAll(status, "_", " ")
		_ = createNotification(h.DB, customerID, "order", "Delivery update", "Your rider updated the delivery: "+label+".", "/orders")
	}
	if status == "delivered" {
		var orderID int
		if h.DB.QueryRow(`SELECT id FROM orders WHERE rider_access_token_hash=$1`, riderTokenHash(token)).Scan(&orderID) == nil {
			if err := h.awardLoyaltyForOrder(orderID); err != nil {
				log.Printf("rider delivered loyalty award: %v", err)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
func (h *Handlers) RiderLocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	token := r.PathValue("token")
	lat, e1 := strconv.ParseFloat(r.FormValue("latitude"), 64)
	lon, e2 := strconv.ParseFloat(r.FormValue("longitude"), 64)
	accuracy, accuracyErr := strconv.ParseFloat(r.FormValue("accuracy"), 64)
	if accuracyErr != nil || math.IsNaN(accuracy) || math.IsInf(accuracy, 0) || accuracy <= 0 || accuracy > 100000 {
		accuracy = 0 // Older rider pages may not send accuracy; coordinates can still be saved.
	}
	if e1 != nil || e2 != nil || math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
		http.Error(w, "Invalid coordinates", 400)
		return
	}
	res, err := h.DB.Exec(`UPDATE orders SET rider_latitude=$1,rider_longitude=$2,rider_location_accuracy=NULLIF($3,0),rider_location_updated_at=NOW(),rider_sharing_location=TRUE WHERE rider_access_token_hash=$4 AND rider_id IS NOT NULL AND status NOT IN ('cancelled','completed') AND EXISTS(SELECT 1 FROM riders rr WHERE rr.id=orders.rider_id AND rr.active=TRUE)`, lat, lon, accuracy, riderTokenHash(token))
	if err != nil {
		http.Error(w, "Could not save location", 500)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "Tracking link is no longer active", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
func (h *Handlers) RiderStopLocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	_, err := h.DB.Exec(`UPDATE orders SET rider_sharing_location=FALSE,rider_latitude=NULL,rider_longitude=NULL,rider_location_accuracy=NULL,rider_location_updated_at=NULL WHERE rider_access_token_hash=$1`, riderTokenHash(r.PathValue("token")))
	if err != nil {
		http.Error(w, "Could not stop sharing", 500)
		return
	}
	w.WriteHeader(204)
}

// CustomerRiderLocation exposes the latest location only to the customer who owns the order.
func (h *Handlers) CustomerRiderLocation(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	orderID, err := strconv.Atoi(r.PathValue("orderID"))
	if err != nil || orderID < 1 {
		http.Error(w, "not found", 404)
		return
	}
	var lat, lon float64
	var available bool
	var deliveryStatus, locationUpdated string
	err = h.DB.QueryRow(`SELECT COALESCE(rider_latitude,0),COALESCE(rider_longitude,0),COALESCE(rider_sharing_location,FALSE),COALESCE(delivery_status,'unassigned'),COALESCE(TO_CHAR(rider_location_updated_at,'DD Mon YYYY, HH24:MI'),'') FROM orders WHERE id=$1 AND user_id=$2 AND delivery_method='delivery'`, orderID, userID).Scan(&lat, &lon, &available, &deliveryStatus, &locationUpdated)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	embed := ""
	if available && (lat != 0 || lon != 0) {
		embed = fmt.Sprintf("https://www.openstreetmap.org/export/embed.html?bbox=%.6f%%2C%.6f%%2C%.6f%%2C%.6f&layer=mapnik&marker=%.6f%%2C%.6f", lon-.01, lat-.01, lon+.01, lat+.01, lat, lon)
	}
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"available": available, "embed_url": embed, "delivery_status": deliveryStatus, "location_updated": locationUpdated})
}
