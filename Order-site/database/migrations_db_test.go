package database_test

import (
	"database/sql"
	"testing"

	"Order-site/database"
	"Order-site/testutil"
)

func versions(t *testing.T, db *sql.DB) []int {
	t.Helper()
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// A brand-new database ends up with every migration recorded, and a second start changes nothing.
func TestMigrationsRecordedAndNotRepeated(t *testing.T) {
	db := testutil.NewDB(t)
	got := versions(t, db)
	if len(got) < 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected migrations 1 and 2 to be recorded, got %v", got)
	}
	var applied string
	if err := db.QueryRow(`SELECT MAX(applied_at)::text FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}

	again, err := database.Connect()
	if err != nil {
		t.Fatalf("second start failed: %v", err)
	}
	defer again.Close()
	var after string
	if err := again.QueryRow(`SELECT MAX(applied_at)::text FROM schema_migrations`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != applied || len(versions(t, again)) != len(got) {
		t.Fatalf("a second start must not re-apply anything (before %s, after %s)", applied, after)
	}
}

// A production database built before numbered migrations existed has all the tables but no
// schema_migrations table. The first start must adopt it without touching its data.
func TestMigrationsAdoptExistingDatabase(t *testing.T) {
	db := testutil.NewDB(t)
	if _, err := db.Exec(`INSERT INTO users (name, email, password) VALUES ('Keep Me', 'keep@example.com', 'x')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE schema_migrations`); err != nil {
		t.Fatal(err)
	}
	adopted, err := database.Connect()
	if err != nil {
		t.Fatalf("adopting an existing database failed: %v", err)
	}
	defer adopted.Close()
	if got := versions(t, adopted); len(got) < 2 {
		t.Fatalf("expected the migrations to be recorded after adoption, got %v", got)
	}
	var n int
	if err := adopted.QueryRow(`SELECT COUNT(*) FROM users WHERE email = 'keep@example.com'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("existing data must survive adoption (count %d, err %v)", n, err)
	}
}

// Two copies of the app starting together must both succeed and apply each migration once.
func TestMigrationsConcurrentStart(t *testing.T) {
	db := testutil.NewDB(t)
	if _, err := db.Exec(`DROP TABLE schema_migrations`); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			c, err := database.Connect()
			if err == nil {
				c.Close()
			}
			errs <- err
		}()
	}
	for i := 0; i < 3; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent start failed: %v", err)
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n < 2 {
		t.Fatalf("each migration should be recorded exactly once (rows %d, err %v)", n, err)
	}
}
