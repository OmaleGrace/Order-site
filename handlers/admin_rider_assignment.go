package handlers

import (
	"Order-site/errors"
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
	if riderID > 0 {
		_, err = h.DB.Exec(`UPDATE orders SET rider_id=$1, delivery_status='assigned' WHERE id=$2 AND delivery_method='delivery'`, riderID, orderID)
	} else {
		_, err = h.DB.Exec(`UPDATE orders SET rider_id=NULL, delivery_status='unassigned' WHERE id=$1 AND delivery_method='delivery'`, orderID)
	}
	if err != nil {
		log.Printf("admin orders: rider assignment failed: %v", err)
	}
	http.Redirect(w, r, "/admin/orders", 303)
}
