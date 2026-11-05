package handlers

import (
	"Order-site/errors"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

func (h *Handlers) AdminAssignRider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Riders are assigned from the admin orders page.", http.StatusMethodNotAllowed)
		return
	}
	orderID, _ := strconv.Atoi(r.FormValue("order_id"))
	riderID, _ := strconv.Atoi(r.FormValue("rider_id"))
	if orderID < 1 {
		http.Redirect(w, r, "/admin/orders", 303)
		return
	}
	var err error
	token := ""
	if riderID > 0 {
		var hash string
		token, hash, err = newRiderToken()
		if err == nil {
			var res sql.Result
			res, err = h.DB.Exec(`UPDATE orders SET rider_id=$1,delivery_status='assigned',rider_access_token_hash=$2,rider_sharing_location=FALSE,rider_latitude=NULL,rider_longitude=NULL,rider_location_accuracy=NULL,rider_location_updated_at=NULL,rider_status_updated_at=NOW() WHERE id=$3 AND delivery_method='delivery' AND status NOT IN ('pending','cancelled','completed') AND EXISTS(SELECT 1 FROM riders WHERE id=$1 AND active=TRUE)`, riderID, hash, orderID)
			if err == nil {
				if n, _ := res.RowsAffected(); n == 0 {
					err = fmt.Errorf("order or active rider not found")
				}
			}
		}
	} else {
		_, err = h.DB.Exec(`UPDATE orders SET rider_id=NULL,delivery_status='unassigned',rider_access_token_hash=NULL,rider_sharing_location=FALSE,rider_latitude=NULL,rider_longitude=NULL,rider_location_accuracy=NULL,rider_location_updated_at=NULL WHERE id=$1 AND delivery_method='delivery'`, orderID)
	}
	if err != nil {
		log.Printf("admin orders: rider assignment failed: %v", err)
		errors.Render(w, "Could not assign rider", 500)
		return
	}
	if token != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		link := riderAccessURL(r, token)
		fmt.Fprintf(w, `<!doctype html><html><body style="font:16px system-ui;max-width:600px;margin:40px auto;padding:20px"><h1>Rider assigned</h1><p>Send this private link directly to the rider. Anyone with it can update this delivery.</p><p><a href="%s">Open rider page</a></p><p><code>%s</code></p><p><a href="/admin/orders">Back to admin orders</a></p></body></html>`, link, link)
		return
	}
	http.Redirect(w, r, "/admin/orders", 303)
}
