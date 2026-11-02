package handlers

import (
	"database/sql"
	"time"
)

// retryDB runs a database read and tries once more if it failed for a reason other than
// "no rows". Hosted databases sometimes drop an idle connection or pause for a moment; the
// second try nearly always succeeds on a fresh connection, so the person never sees an error.
// Only use it for reads, or for writes that are safe to repeat.
func retryDB(fn func() error) error {
	err := fn()
	if err == nil || err == sql.ErrNoRows {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	return fn()
}
