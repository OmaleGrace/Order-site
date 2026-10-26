package handlers

import (
	"database/sql"
	"log"
	"strconv"
	"time"
)

// scheduledOrderBounds returns the allowed scheduling window.
// The application accepts scheduled times at least 15 minutes from now
// and no more than 30 days ahead.
func scheduledOrderBounds(now time.Time) (time.Time, time.Time) {
	return now.Add(15 * time.Minute), now.Add(30 * 24 * time.Hour)
}

// ActivateScheduledOrders moves paid scheduled orders into the normal paid
// state when their requested fulfilment time arrives. The UPDATE is atomic,
// so repeated scheduler ticks cannot activate an order twice.
// notifyVendorScheduledOrder alerts the vendor owner as soon as a paid order is scheduled.
func notifyVendorScheduledOrder(db *sql.DB, orderID int) error {
	var ownerUserID int
	var vendorName string
	var scheduledFor time.Time
	if err := db.QueryRow(`
		SELECT v.owner_user_id, v.name, o.scheduled_for
		FROM orders o
		JOIN vendors v ON v.id = o.vendor_id
		WHERE o.id = $1 AND o.status = 'scheduled'
	`, orderID).Scan(&ownerUserID, &vendorName, &scheduledFor); err != nil {
		return err
	}
	message := "Order #" + strconv.Itoa(orderID) + " from " + vendorName + " is scheduled for " + scheduledFor.Format("02 Jan 2006, 3:04 PM") + "."
	return createNotification(db, ownerUserID, "vendor_order", "New scheduled order", message, "/vendor/orders")
}

func (h *Handlers) ActivateScheduledOrders() {
	rows, err := h.DB.Query(`
        UPDATE orders
        SET status = 'paid'
        WHERE status = 'scheduled'
          AND scheduled_for IS NOT NULL
          AND scheduled_for <= NOW()
        RETURNING id, user_id
    `)
	if err != nil {
		log.Printf("scheduled orders: activation failed: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var orderID, userID int
		if err := rows.Scan(&orderID, &userID); err != nil {
			log.Printf("scheduled orders: activation scan failed: %v", err)
			continue
		}
		if err := createNotification(h.DB, userID, "order", "Scheduled order is now active", "Your scheduled order is now active and ready for the vendor to prepare.", "/orders"); err != nil {
			log.Printf("scheduled orders: notification failed for order %d: %v", orderID, err)
		}
		go func(id int) {
			if err := sendOrderPlacedEmail(h.DB, id); err != nil {
				log.Printf("scheduled orders: email failed for order %d: %v", id, err)
			}
		}(orderID)
	}
}
