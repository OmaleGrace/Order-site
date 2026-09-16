package menu

import (
	"database/sql"
	"strconv"
)

type MenuItem struct {
	ID          int
	Name        string
	Description string
	PriceKobo   int
	ImageURL    sql.NullString
	Category    sql.NullString
}

func GetAll(db *sql.DB, search, category string) ([]MenuItem, error) {
	query := `
		SELECT id, name, description, price_kobo, image_url, category
		FROM menu_items
		WHERE 1=1
	`
	var args []interface{}
	argIndex := 1

	if search != "" {
		query += " AND (name ILIKE $" + strconv.Itoa(argIndex) + " OR description ILIKE $" + strconv.Itoa(argIndex) + ")"
		args = append(args, "%"+search+"%")
		argIndex++
	}

	if category != "" {
		query += " AND category = $" + strconv.Itoa(argIndex)
		args = append(args, category)
		argIndex++
	}

	query += " ORDER BY id"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []MenuItem{}

	for rows.Next() {
		var item MenuItem
		err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Description,
			&item.PriceKobo,
			&item.ImageURL,
			&item.Category,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
