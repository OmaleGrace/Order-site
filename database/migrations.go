package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
)

// Numbered migrations.
//
// Migration 1 is the baseline (everything that existed before this system). Every later change is
// a file in database/migrations named NNNN_short_name.sql, for example 0003_add_wallet_note.sql.
// Each file runs once, inside a transaction, in number order, and is recorded in schema_migrations.
// Never edit a file that has been released: add a new one that changes things further.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationLockID is an arbitrary number that identifies "a migration is running". It makes sure
// two copies of the app starting at the same moment do not run migrations at the same time.
const migrationLockID = 727002

// execer is what the baseline needs: both *sql.DB and *sql.Tx can run statements this way.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

type migration struct {
	version int
	name    string
	run     func(tx *sql.Tx) error
}

// parseMigrationName reads "0003_add_wallet_note.sql" into 3 and "add_wallet_note".
func parseMigrationName(file string) (int, string, error) {
	base := strings.TrimSuffix(file, ".sql")
	num, name, ok := strings.Cut(base, "_")
	if !ok || name == "" {
		return 0, "", fmt.Errorf("migration file %q must be named NNNN_name.sql", file)
	}
	v, err := strconv.Atoi(num)
	if err != nil || v < 2 {
		return 0, "", fmt.Errorf("migration file %q must start with a number of 2 or more (1 is the built-in baseline)", file)
	}
	return v, name, nil
}

// loadMigrations returns the baseline plus every embedded SQL file, in order.
func loadMigrations() ([]migration, error) {
	list := []migration{{version: 1, name: "baseline", run: func(tx *sql.Tx) error { return baselineMigration(tx) }}}
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	seen := map[int]string{1: "baseline"}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		v, name, err := parseMigrationName(e.Name())
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[v]; dup {
			return nil, fmt.Errorf("two migrations share the number %d (%s and %s)", v, prev, e.Name())
		}
		seen[v] = e.Name()
		body, err := migrationFiles.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		query := string(body)
		list = append(list, migration{version: v, name: name, run: func(tx *sql.Tx) error {
			_, err := tx.Exec(query)
			return err
		}})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].version < list[j].version })
	return list, nil
}

// migrate applies every migration that has not run on this database yet.
//
// Each migration runs in its own transaction that also records it in schema_migrations, so it
// either happens completely or not at all. A transaction-level lock (not a session-level one,
// which hosted connection poolers do not support) stops two copies of the app that start at the
// same moment from applying the same migration twice.
func migrate(db *sql.DB) error {
	list, err := loadMigrations()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if err := inLockedTx(ctx, db, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`)
		return err
	}); err != nil {
		return err
	}

	for _, m := range list {
		var done bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, m.version).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		err := inLockedTx(ctx, db, func(tx *sql.Tx) error {
			// Another copy of the app may have finished this one while we waited for the lock.
			if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, m.version).Scan(&done); err != nil {
				return err
			}
			if done {
				return nil
			}
			log.Printf("database: applying migration %04d %s", m.version, m.name)
			if err := m.run(tx); err != nil {
				return err
			}
			_, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.version, m.name)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %04d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

// inLockedTx runs fn in a transaction that holds the migration lock until it commits.
func inLockedTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, migrationLockID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
