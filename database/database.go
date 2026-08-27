package database

import (
	"database/sql"
	"os"

	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	databaseURL := os.Getenv("DATABASE_URL")

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

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
		return nil, err
	}

	_, err = db.Exec(`
    ALTER TABLE menu_items
    ADD COLUMN IF NOT EXISTS image_url TEXT
`)
	if err != nil {
		return nil, err
	}

	return db, nil
}
