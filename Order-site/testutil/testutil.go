package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// loadSchema reads schema.sql and drops the pg_dump lines that only psql understands.
func loadSchema() (string, error) {
	root, err := projectRoot()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(root, "schema.sql"))
	if err != nil {
		return "", err
	}

	var keep []string
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, `\`),
			strings.HasPrefix(t, "--"),
			strings.HasPrefix(t, "SET "),
			strings.HasPrefix(t, "SELECT pg_catalog.set_config"),
			strings.HasPrefix(t, "CREATE SCHEMA"),
			strings.HasPrefix(t, "COMMENT ON SCHEMA"):
			continue
		}
		keep = append(keep, line)
	}
	return strings.Join(keep, "\n"), nil
}

// NewDB gives a test a clean database built from schema.sql, with vendor 1 (an active
// shop) already in place. Only one test uses the database at a time.
func NewDB(t *testing.T) *sql.DB {
	t.Helper()

	raw := testURL()
	if err := safeToReset(raw); err != nil {
		t.Fatal(err)
	}

	schema, err := loadSchema()
	if err != nil {
		t.Fatalf("reading schema.sql: %v", err)
	}

	db, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
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
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("loading schema.sql: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO vendors (id, name, slug, status, setup_paid_at, paid_until)
		VALUES (1, 'Test Kitchen', 'test-kitchen', 'active', NOW(), NOW() + INTERVAL '100 years');
		SELECT setval(pg_get_serial_sequence('vendors', 'id'), 1);
	`); err != nil {
		t.Fatalf("seeding the test vendor: %v", err)
	}

	return db
}
