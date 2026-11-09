package alerts

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"Order-site/safe"
)

// WatchDB pings the database every minute. After three failures in a row it sends one alert, and
// one "recovered" notice when it comes back. It stops when stop is closed.
func WatchDB(db *sql.DB, stop <-chan struct{}) {
	go func() {
		defer safe.Recover("database watch")
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		fails, down := 0, false
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := db.PingContext(ctx)
				cancel()
				if err == nil {
					if down {
						Notify("db-up", "Database is back", "The site can reach its database again.")
					}
					fails, down = 0, false
					continue
				}
				fails++
				log.Printf("alerts: database ping failed (%d in a row): %v", fails, err)
				if fails >= 3 && !down {
					down = true
					Notify("db-down", "Database is not responding", fmt.Sprintf("3 checks in a row failed. Last error: %v", err))
				}
			}
		}
	}()
}
