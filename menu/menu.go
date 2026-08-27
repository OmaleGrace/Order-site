package menu

import "database/sql"

type MenuItem struct {
	ID          int
	Name        string
	Description string
	PriceKobo   int
	ImageURL string
}

func GetAll(db *sql.DB) ([]MenuItem, error) {
	rows, err := db.Query(`
	SELECT id, name, description, price_kobo, image_url
	FROM menu_items
	ORDER BY id
	`)
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
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
