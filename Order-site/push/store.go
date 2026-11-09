package push

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"
)

// Save stores (or moves to this person) the subscription for one device.
func Save(db *sql.DB, userID int, s Subscription, userAgent string) error {
	if len(userAgent) > 300 {
		userAgent = userAgent[:300]
	}
	_, err := db.Exec(`
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE
		SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth,
		    user_agent = EXCLUDED.user_agent, fail_count = 0`,
		userID, s.Endpoint, s.P256dh, s.Auth, userAgent)
	return err
}

// Remove deletes one device's subscription (only if it belongs to userID).
func Remove(db *sql.DB, userID int, endpoint string) error {
	_, err := db.Exec(`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`, userID, endpoint)
	return err
}

// Count is how many devices will receive notifications for this person.
func Count(db *sql.DB, userID int) int {
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM push_subscriptions WHERE user_id = $1`, userID).Scan(&n)
	return n
}

// Result says what happened when sending to one person.
type Result struct {
	Devices int
	Sent    int
	Removed int
	Failed  int
}

// NotifyUser sends p to every device of one person and tidies up dead subscriptions. It is safe
// to call in a goroutine. When push is not configured it does nothing.
func NotifyUser(db *sql.DB, userID int, p Payload) Result {
	var res Result
	if !Enabled() {
		return res
	}
	rows, err := db.Query(`SELECT id, endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID)
	if err != nil {
		log.Printf("push: could not load subscriptions for user %d: %v", userID, err)
		return res
	}
	type row struct {
		id  int
		sub Subscription
	}
	var subs []row
	for rows.Next() {
		var r row
		if rows.Scan(&r.id, &r.sub.Endpoint, &r.sub.P256dh, &r.sub.Auth) == nil {
			subs = append(subs, r)
		}
	}
	rows.Close()
	res.Devices = len(subs)

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, r := range subs {
		wg.Add(1)
		go func(r row) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			err := Send(ctx, r.sub, p)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				res.Sent++
				_, _ = db.Exec(`UPDATE push_subscriptions SET last_success_at = NOW(), fail_count = 0 WHERE id = $1`, r.id)
			case err == ErrGone:
				res.Removed++
				_, _ = db.Exec(`DELETE FROM push_subscriptions WHERE id = $1`, r.id)
			default:
				res.Failed++
				log.Printf("push: sending to user %d failed: %v", userID, err)
				// A device that keeps failing is dropped after 10 tries in a row.
				_, _ = db.Exec(`UPDATE push_subscriptions SET fail_count = fail_count + 1 WHERE id = $1`, r.id)
				_, _ = db.Exec(`DELETE FROM push_subscriptions WHERE id = $1 AND fail_count >= 10`, r.id)
			}
		}(r)
	}
	wg.Wait()
	return res
}
