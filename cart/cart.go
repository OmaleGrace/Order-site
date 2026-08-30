package cart

import "database/sql"

type CartItem struct {
	MenuItemID  int
	Name        string
	Description string
	Quantity    int
	PriceKobo   int
}

func AddItem(db *sql.DB, userID, menuItemID int) (bool, error) {
	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM cart_items WHERE user_id = $1 AND menu_item_id = $2)",
		userID, menuItemID,
	).Scan(&exists)
	if err != nil {
		return false, err
	}

	if exists {
		return false, nil
	}

	var priceKobo int
	err = db.QueryRow("SELECT price_kobo FROM menu_items WHERE id = $1", menuItemID).Scan(&priceKobo)
	if err != nil {
		return false, err
	}

	_, err = db.Exec(`
		INSERT INTO cart_items (user_id, menu_item_id, quantity, price_kobo_at_addition)
		VALUES ($1, $2, 1, $3)
	`, userID, menuItemID, priceKobo)
	return true, err
}

func RemoveItem(db *sql.DB, userID, menuItemID int) error {
	_, err := db.Exec("DELETE FROM cart_items WHERE user_id = $1 AND menu_item_id = $2", userID, menuItemID)
	return err
}

func GetItems(db *sql.DB, userID int) ([]CartItem, error) {
	rows, err := db.Query(`
		SELECT ci.menu_item_id, ci.quantity, ci.price_kobo_at_addition, m.name, m.description
		FROM cart_items ci
		JOIN menu_items m ON m.id = ci.menu_item_id
		WHERE ci.user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var item CartItem
		err := rows.Scan(&item.MenuItemID, &item.Quantity, &item.PriceKobo, &item.Name, &item.Description)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func ClearCart(db *sql.DB, userID int) error {
	_, err := db.Exec(`
		DELETE FROM cart_items
		WHERE user_id = $1
	`, userID)

	return err
}