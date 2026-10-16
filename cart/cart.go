package cart

import "database/sql"

type CartItem struct {
	MenuItemID  int
	Name        string
	Description string
	Quantity    int
	PriceKobo   int
	ImageURL    string
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

	// Item is already in the cart.
	// Don't increase the quantity.
	if exists {
		return false, nil
	}

	var priceKobo int

	err = db.QueryRow(
		"SELECT m.price_kobo FROM menu_items m JOIN vendors v ON v.id = m.vendor_id WHERE m.id = $1 AND v.status = 'active' AND v.paid_until > NOW()",
		menuItemID,
	).Scan(&priceKobo)

	if err != nil {
		return false, err
	}

	_, err = db.Exec(`
		INSERT INTO cart_items
		(user_id, menu_item_id, quantity, price_kobo_at_addition)
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
		SELECT ci.menu_item_id, ci.quantity, ci.price_kobo_at_addition, m.name, COALESCE(m.description, ''), COALESCE(m.image_url, '')
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
		err := rows.Scan(&item.MenuItemID, &item.Quantity, &item.PriceKobo, &item.Name, &item.Description, &item.ImageURL)
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
// OtherVendorInCart returns the name of a different vendor whose dishes are
// already in the cart, or "" when the cart is empty or all from the same vendor.
func OtherVendorInCart(db *sql.DB, userID, menuItemID int) (string, error) {
	var name string
	err := db.QueryRow(`
		SELECT v.name
		FROM cart_items ci
		JOIN menu_items m ON m.id = ci.menu_item_id
		JOIN vendors v ON v.id = m.vendor_id
		WHERE ci.user_id = $1
		  AND m.vendor_id <> (SELECT vendor_id FROM menu_items WHERE id = $2)
		LIMIT 1
	`, userID, menuItemID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return name, err
}
