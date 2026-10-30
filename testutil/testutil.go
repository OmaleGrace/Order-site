package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"Order-site/database"

	_ "github.com/lib/pq"
)

const defaultTestURL = "postgresql://testuser:testpass@localhost:5433/food_ordering_test?sslmode=disable"

func testURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return defaultTestURL
}

// safeToReset refuses to touch anything that is not a local database whose name ends in _test.
func safeToReset(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
	default:
		return fmt.Errorf("refusing to reset a database on %q: tests may only use a local database", u.Hostname())
	}
	name := strings.TrimPrefix(u.Path, "/")
	if !strings.HasSuffix(name, "_test") {
		return fmt.Errorf("refusing to reset database %q: its name must end in _test", name)
	}
	return nil
}

// NewDB gives a test a clean database built the way the app builds it (schema + migrations), with vendor 1 (an active
// shop) already in place. Only one test uses the database at a time.
func NewDB(t *testing.T) *sql.DB {
	t.Helper()

	raw := testURL()
	if err := safeToReset(raw); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal(err)
	}
	// Closed last (cleanups run in reverse order), after the lock below is released.
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("cannot reach the test database (is the food-test-db container running?): %v", err)
	}

	ctx := context.Background()
	lock, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := lock.ExecContext(ctx, "SELECT pg_advisory_lock(727001)"); err != nil {
		lock.Close()
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		lock.ExecContext(context.Background(), "SELECT pg_advisory_unlock(727001)")
		lock.Close()
	})

	if _, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("resetting the test database: %v", err)
	}
	// Build the database exactly the way the app does at start-up: schema first, then every
	// migration. Loading schema.sql alone leaves out tables that only migrations create
	// (support_tickets, coupons, riders, ...), so tests would not match the real app.
	t.Setenv("DATABASE_URL", raw)
	migrated, err := database.Connect()
	if err != nil {
		t.Fatalf("building the test database like the app does: %v", err)
	}
	t.Cleanup(func() { migrated.Close() })
	db = migrated
	if _, err := db.Exec(`
		INSERT INTO vendors (id, name, slug, status, setup_paid_at, paid_until)
		VALUES (1, 'Test Kitchen', 'test-kitchen', 'active', NOW(), NOW() + INTERVAL '100 years');
		SELECT setval(pg_get_serial_sequence('vendors', 'id'), 1);
	`); err != nil {
		t.Fatalf("seeding the test vendor: %v", err)
	}

	return db
}
