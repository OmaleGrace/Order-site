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
	VendorID    int
	VendorName  string
	VendorSlug  string
}

// LiveVendorSQL is true for a vendor whose dishes customers may see:
// active status and a paid period that has not run out.
const LiveVendorSQL = "v.status = 'active' AND v.paid_until > NOW()"

func GetAll(db *sql.DB, search, category string) ([]MenuItem, error) {
	return GetAllWithOptions(db, search, category, "relevance", 0, 0)
}

func GetAllWithOptions(db *sql.DB, search, category, sortBy string, minPriceKobo, maxPriceKobo int) ([]MenuItem, error) {
	query := `
		SELECT m.id, m.name, COALESCE(m.description, ''), m.price_kobo,
		       m.image_url, m.category, v.id, v.name, v.slug
		FROM menu_items m
		JOIN vendors v ON v.id = m.vendor_id
		WHERE ` + LiveVendorSQL + ` AND COALESCE(m.available, TRUE) = TRUE
	`
	var args []interface{}
	argIndex := 1

	if search != "" {
		query += " AND (m.name ILIKE $" + strconv.Itoa(argIndex) +
			" OR m.description ILIKE $" + strconv.Itoa(argIndex) +
			" OR v.name ILIKE $" + strconv.Itoa(argIndex) + ")"
		args = append(args, "%"+search+"%")
		argIndex++
	}

	if category != "" {
		query += " AND m.category = $" + strconv.Itoa(argIndex)
		args = append(args, category)
		argIndex++
	}

	if minPriceKobo > 0 {
		query += " AND m.price_kobo >= $" + strconv.Itoa(argIndex)
		args = append(args, minPriceKobo)
		argIndex++
	}
	if maxPriceKobo > 0 {
		query += " AND m.price_kobo <= $" + strconv.Itoa(argIndex)
		args = append(args, maxPriceKobo)
		argIndex++
	}

	switch sortBy {
	case "price_asc":
		query += " ORDER BY m.price_kobo ASC, m.name ASC"
	case "price_desc":
		query += " ORDER BY m.price_kobo DESC, m.name ASC"
	case "name":
		query += " ORDER BY m.name ASC"
	default:
		query += " ORDER BY m.id"
	}

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
			&item.VendorID,
			&item.VendorName,
			&item.VendorSlug,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
