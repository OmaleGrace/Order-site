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

// LiveVendorSQL is true for a vendor whose dishes customers may see:
// active status and a paid period that has not run out.
const LiveVendorSQL = "v.status = 'active' AND v.paid_until > NOW()"

func GetAll(db *sql.DB, search, category string) ([]MenuItem, error) {
	query := `
		SELECT m.id, m.name, COALESCE(m.description, ''), m.price_kobo, m.image_url, m.category
		FROM menu_items m
		JOIN vendors v ON v.id = m.vendor_id
		WHERE ` + LiveVendorSQL + `
	`
	var args []interface{}
	argIndex := 1

	if search != "" {
		query += " AND (m.name ILIKE $" + strconv.Itoa(argIndex) + " OR m.description ILIKE $" + strconv.Itoa(argIndex) + ")"
		args = append(args, "%"+search+"%")
		argIndex++
	}

	if category != "" {
		query += " AND m.category = $" + strconv.Itoa(argIndex)
		args = append(args, category)
		argIndex++
	}

	query += " ORDER BY m.id"

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
	return items, rows.Err()
}