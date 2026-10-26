package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
)

type LoyaltyTransaction struct {
	Points      int
	Description string
	Reference   string
	CreatedAt   string
}

type LoyaltyPageData struct {
	Points       int
	Lifetime     int
	Tier         string
	NextTier     string
	NextTierAt   int
	Progress     int
	Transactions []LoyaltyTransaction
}

func loyaltyTier(points int) (tier, next string, nextAt int, progress int) {
	switch {
	case points >= 3000:
		return "Platinum", "Platinum", 3000, 100
	case points >= 1500:
		return "Gold", "Platinum", 3000, (points * 100) / 3000
	case points >= 500:
		return "Silver", "Gold", 1500, (points * 100) / 1500
	default:
		return "Bronze", "Silver", 500, (points * 100) / 500
	}
}

func (h *Handlers) Loyalty(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		errors.Render(w, "Please sign in to view your loyalty rewards.", http.StatusUnauthorized)
		return
	}

	var data LoyaltyPageData
	if err := h.DB.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN type = 'earned' THEN points ELSE -points END), 0),
		       COALESCE(SUM(CASE WHEN type = 'earned' THEN points ELSE 0 END), 0)
		FROM loyalty_transactions
		WHERE user_id = $1
	`, userID).Scan(&data.Points, &data.Lifetime); err != nil {
		errors.Render(w, "Could not load loyalty points", http.StatusInternalServerError)
		return
	}

	data.Tier, data.NextTier, data.NextTierAt, data.Progress = loyaltyTier(data.Lifetime)
	if data.Progress > 100 {
		data.Progress = 100
	}

	rows, err := h.DB.Query(`
		SELECT points, description, COALESCE(order_id, 0), created_at
		FROM loyalty_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		errors.Render(w, "Could not load loyalty history", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var points, orderID int
		var description string
		var createdAt time.Time
		if err := rows.Scan(&points, &description, &orderID, &createdAt); err != nil {
			errors.Render(w, "Could not read loyalty history", http.StatusInternalServerError)
			return
		}
		reference := ""
		if orderID > 0 {
			reference = publicOrderReference(orderID, createdAt)
		}
		if description == "" {
			description = "Loyalty points activity"
		}
		data.Transactions = append(data.Transactions, LoyaltyTransaction{
			Points:      points,
			Description: description,
			Reference:   reference,
			CreatedAt:   createdAt.Format("02 Jan 2006, 3:04 PM"),
		})
	}
	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read loyalty history", http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(brand.ParseE("templates/loyalty.html"))
	if err := tmpl.Execute(w, data); err != nil {
		errors.Render(w, "Could not render loyalty page", http.StatusInternalServerError)
	}
}

// awardLoyaltyForOrder gives one point for every ₦100 of food purchased on a
// completed order. The earned-order unique index makes the award idempotent.
func (h *Handlers) awardLoyaltyForOrder(orderID int) error {
	var userID, foodTotal int
	if err := h.DB.QueryRow(`
		SELECT user_id, GREATEST(total_kobo - delivery_fee_kobo, 0)
		FROM orders
		WHERE id = $1 AND status = 'completed'
	`, orderID).Scan(&userID, &foodTotal); err != nil {
		return err
	}

	points := foodTotal / 10000 // 10000 kobo = ₦100 = 1 point
	if points <= 0 {
		return nil
	}

	_, err := h.DB.Exec(`
		INSERT INTO loyalty_transactions (user_id, order_id, points, type, description)
		VALUES ($1, $2, $3, 'earned', 'Points earned from completed order')
		ON CONFLICT DO NOTHING
	`, userID, orderID, points)
	return err
}

// redeemLoyaltyForOrder records the points used on a paid order. The caller
// must already be inside the payment transaction. It locks the user's loyalty
// rows so concurrent payment callbacks cannot spend the same points twice.
func redeemLoyaltyForOrder(tx *sql.Tx, userID, orderID, points int) error {
	if points <= 0 {
		return nil
	}

	// Lock this user's loyalty rows for the duration of the payment transaction.
	rows, err := tx.Query(`SELECT id FROM loyalty_transactions WHERE user_id = $1 FOR UPDATE`, userID)
	if err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	var available int
	if err := tx.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN type = 'earned' THEN points ELSE -points END), 0)
		FROM loyalty_transactions
		WHERE user_id = $1
	`, userID).Scan(&available); err != nil {
		return err
	}
	if points > available {
		return fmt.Errorf("not enough loyalty points")
	}

	_, err = tx.Exec(`
		INSERT INTO loyalty_transactions (user_id, order_id, points, type, description)
		VALUES ($1, $2, $3, 'redeemed', 'Loyalty points used on order')
	`, userID, orderID, points)
	return err
}
