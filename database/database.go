package database

import (
	"database/sql"
	"os"

	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	databaseURL := os.Getenv("postgresql://gracie_ordering_user:fRLnSNI6lM8XacVteOR0AFsTaO0s8LJf@dpg-da3fvbht0dsc73fj80ag-a/gracie_ordering")

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}
