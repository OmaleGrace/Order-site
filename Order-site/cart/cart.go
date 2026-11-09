package cart

import (
	"database/sql"
	"fmt"
)

type CartOption struct {
	ID        int
	Name      string
	PriceKobo int
}

type CartItem struct {
	MenuItemID  int
	Name        string
	Description string
	Quantity    int
	PriceKobo   int
	ImageURL    string
	Options     []CartOption
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
		SELECT ci.id, ci.menu_item_id, ci.quantity, ci.price_kobo_at_addition, m.name, COALESCE(m.description, ''), COALESCE(m.image_url, '')
		FROM cart_items ci
		JOIN menu_items m ON m.id = ci.menu_item_id
		WHERE ci.user_id = $1
		ORDER BY ci.id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CartItem{}
	cartIDs := []int{}
	for rows.Next() {
		var item CartItem
		var cartID int
		if err := rows.Scan(&cartID, &item.MenuItemID, &item.Quantity, &item.PriceKobo, &item.Name, &item.Description, &item.ImageURL); err != nil {
			return nil, err
		}
		items = append(items, item)
		cartIDs = append(cartIDs, cartID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, cartID := range cartIDs {
		optionRows, err := db.Query(`SELECT option_id, name_at_addition, price_kobo_at_addition FROM cart_item_options WHERE cart_item_id=$1 ORDER BY id`, cartID)
		if err != nil {
			return nil, err
		}
		for optionRows.Next() {
			var o CartOption
			if err := optionRows.Scan(&o.ID, &o.Name, &o.PriceKobo); err != nil {
				optionRows.Close()
				return nil, err
			}
			items[i].Options = append(items[i].Options, o)
			items[i].PriceKobo += o.PriceKobo
		}
		if err := optionRows.Err(); err != nil {
			optionRows.Close()
			return nil, err
		}
		optionRows.Close()
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

// AddItemWithOptions adds a dish and snapshots the selected customization prices.
// The existing cart model keeps one row per dish, so a dish can have one selected
// customization set at a time; customers can change it by removing and re-adding.
func AddItemWithOptions(db *sql.DB, userID, menuItemID int, optionIDs []int) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM cart_items WHERE user_id=$1 AND menu_item_id=$2)`, userID, menuItemID).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	var price int
	if err := tx.QueryRow(`SELECT m.price_kobo FROM menu_items m JOIN vendors v ON v.id=m.vendor_id WHERE m.id=$1 AND v.status='active' AND v.paid_until>NOW()`, menuItemID).Scan(&price); err != nil {
		return false, err
	}

	// Load active choices and enforce each group's min/max selection rules.
	type opt struct {
		groupID int
		name    string
		price   int
	}
	valid := map[int]opt{}
	mins, maxs, counts := map[int]int{}, map[int]int{}, map[int]int{}
	rows, err := tx.Query(`SELECT g.id,g.min_select,g.max_select,o.id,o.name,o.price_kobo FROM menu_option_groups g JOIN menu_options o ON o.group_id=g.id WHERE g.menu_item_id=$1 AND g.active=TRUE AND o.active=TRUE`, menuItemID)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var gid, minS, maxS, oid, p int
		var name string
		if err := rows.Scan(&gid, &minS, &maxS, &oid, &name, &p); err != nil {
			rows.Close()
			return false, err
		}
		valid[oid] = opt{gid, name, p}
		mins[gid] = minS
		maxs[gid] = maxS
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	seen := map[int]bool{}
	for _, oid := range optionIDs {
		if seen[oid] {
			continue
		}
		seen[oid] = true
		o, ok := valid[oid]
		if !ok {
			return false, fmt.Errorf("invalid customization option")
		}
		counts[o.groupID]++
		if counts[o.groupID] > maxs[o.groupID] {
			return false, fmt.Errorf("too many customization choices selected")
		}
	}
	for gid, minS := range mins {
		if counts[gid] < minS {
			return false, fmt.Errorf("please choose all required customization options")
		}
	}

	var cartID int
	if err := tx.QueryRow(`INSERT INTO cart_items (user_id,menu_item_id,quantity,price_kobo_at_addition) VALUES ($1,$2,1,$3) RETURNING id`, userID, menuItemID, price).Scan(&cartID); err != nil {
		return false, err
	}
	for _, oid := range optionIDs {
		if !seen[oid] {
			continue
		}
		o := valid[oid]
		if _, err := tx.Exec(`INSERT INTO cart_item_options (cart_item_id,option_id,name_at_addition,price_kobo_at_addition) VALUES ($1,$2,$3,$4)`, cartID, oid, o.name, o.price); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
