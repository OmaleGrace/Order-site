package middleware

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"Order-site/safe"
)

// ActiveWindow is how recently someone must have been active to count as "active now".
const ActiveWindow = 2 * time.Minute

var (
	lastTouch   sync.Map // userID -> time.Time of the last write for that user
	touchEvery  = time.Minute
	touchMaxAge = 30 * time.Minute
)

// TouchLastSeen records that a signed-in member is using the site right now. It writes to the
// database at most once a minute per member, and never for guests, so it adds almost no load.
func TouchLastSeen(db *sql.DB, userID int) {
	if userID < 1 {
		return
	}
	now := time.Now()
	if v, ok := lastTouch.Load(userID); ok && now.Sub(v.(time.Time)) < touchEvery {
		return
	}
	lastTouch.Store(userID, now)
	go func() {
		defer safe.Recover("record user activity")
		if _, err := db.Exec(`UPDATE users SET last_seen_at = NOW() WHERE id = $1 AND NOT is_guest`, userID); err != nil {
			log.Printf("presence: could not record activity: %v", err)
		}
	}()
	if now.Second()%7 == 0 { // now and then, forget members who have not been seen for a while
		lastTouch.Range(func(k, v any) bool {
			if now.Sub(v.(time.Time)) > touchMaxAge {
				lastTouch.Delete(k)
			}
			return true
		})
	}
}
