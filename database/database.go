package database

import (
	"database/sql"
	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	db, err := sql.Open(
		"postgres",
		"host=127.0.0.1 port=5432 user=food_app password=food_password dbname=food_ordering sslmode=disable",
	)

	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}