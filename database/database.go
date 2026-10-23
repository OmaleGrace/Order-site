package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not reach PostgreSQL using DATABASE_URL: %w", err)
	}

	// A newly-created hosted PostgreSQL database has no application tables.
	// Bootstrap the checked-in schema only when the core users table is absent;
	// never reapply the schema dump to an existing database.
	var usersTable sql.NullString
	if err := db.QueryRow(`SELECT to_regclass('public.users')`).Scan(&usersTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not inspect database schema: %w", err)
	}
	if !usersTable.Valid {
		if err := bootstrapSchema(db); err != nil {
			db.Close()
			return nil, fmt.Errorf("database is empty and schema bootstrap failed: %w", err)
		}
	}

	// Create cart_items table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cart_items (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			menu_item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
			quantity INTEGER NOT NULL DEFAULT 1,
			price_kobo_at_addition INTEGER NOT NULL,
			UNIQUE (user_id, menu_item_id)
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Add image_url column to menu_items if it doesn't exist
	_, err = db.Exec(`
		ALTER TABLE menu_items
		ADD COLUMN IF NOT EXISTS image_url TEXT
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Create orders table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			total_kobo INTEGER NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			payment_reference VARCHAR(100),
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Create order_items table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS order_items (
			id SERIAL PRIMARY KEY,
			order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			menu_item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
			quantity INTEGER NOT NULL,
			price_kobo INTEGER NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Add admin role to users
	_, err = db.Exec(`
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Create sessions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// The platform's cut of each order, fixed when the order is placed
	_, err = db.Exec(`
		ALTER TABLE orders
		ADD COLUMN IF NOT EXISTS commission_kobo INTEGER NOT NULL DEFAULT 0
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Money sent to vendors from their sales, recorded by the admin
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vendor_payouts (
			id SERIAL PRIMARY KEY,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id),
			amount_kobo INTEGER NOT NULL CHECK (amount_kobo > 0),
			note TEXT NOT NULL DEFAULT '',
			paid_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			recorded_by INTEGER
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Store the exact Nigerian state/LGA combinations each vendor serves.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vendor_delivery_areas (
			id SERIAL PRIMARY KEY,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			state TEXT NOT NULL,
			lga TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (vendor_id, state, lga)
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_vendor_delivery_areas_vendor ON vendor_delivery_areas(vendor_id)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Remembers which phone number a text-message code was sent to, and for whom,
	// so a code can only verify the number it was really sent to.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS phone_verifications (
			pin_id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			phone TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// bootstrapSchema applies schema.sql to a truly empty database. The checked-in
// file is a pg_dump schema export, so psql-only meta commands and session setup
// statements are removed before sending the SQL through database/sql.
func bootstrapSchema(db *sql.DB) error {
	data, err := os.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema.sql from the project root: %w", err)
	}
	schemaSQL := cleanSchemaDump(string(data))
	if schemaSQL == "" {
		return fmt.Errorf("schema.sql contains no SQL")
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return err
	}
	return nil
}

func cleanSchemaDump(dump string) string {
	var statements []string
	for _, line := range strings.Split(dump, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, `\`),
			strings.HasPrefix(trimmed, "--"),
			strings.HasPrefix(trimmed, "SET "),
			strings.HasPrefix(trimmed, "SELECT pg_catalog.set_config"),
			strings.HasPrefix(trimmed, "CREATE SCHEMA public"),
			strings.HasPrefix(trimmed, "COMMENT ON SCHEMA public"):
			continue
		}
		statements = append(statements, line)
	}
	return strings.TrimSpace(strings.Join(statements, "\n"))
}
