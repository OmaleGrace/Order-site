package cart

import (
	"Order-site/testutil"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

func setupCartTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// The full schema (including migrations), so these tests match the real app.
	return testutil.NewDB(t)
}

func createCartUser(t *testing.T, db *sql.DB) int {
	t.Helper()

	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Cart User', 'cart@example.com', 'hashed-password')
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	return userID
}

func createMenuItem(t *testing.T, db *sql.DB) int {
	t.Helper()

	var menuItemID int

	err := db.QueryRow(`
		INSERT INTO menu_items (name, description, price_kobo)
		VALUES ('Jollof Rice', 'Delicious jollof rice', 350000)
		RETURNING id
	`).Scan(&menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	return menuItemID
}

func TestAddItem(t *testing.T) {
	db := setupCartTestDB(t)
	defer db.Close()

	userID := createCartUser(t, db)
	menuItemID := createMenuItem(t, db)

	added, err := AddItem(db, userID, menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	if !added {
		t.Fatal("expected item to be added")
	}

	var quantity int
	var price int

	err = db.QueryRow(`
		SELECT quantity, price_kobo_at_addition
		FROM cart_items
		WHERE user_id = $1 AND menu_item_id = $2
	`, userID, menuItemID).Scan(&quantity, &price)

	if err != nil {
		t.Fatal(err)
	}

	if quantity != 1 {
		t.Fatalf("expected quantity 1, got %d", quantity)
	}

	if price != 350000 {
		t.Fatalf("expected price 350000, got %d", price)
	}
}

func TestAddDuplicateItem(t *testing.T) {
	db := setupCartTestDB(t)
	defer db.Close()

	userID := createCartUser(t, db)
	menuItemID := createMenuItem(t, db)

	_, err := AddItem(db, userID, menuItemID)
	if err != nil {
		t.Fatal(err)
	}

	added, err := AddItem(db, userID, menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	if added {
		t.Fatal("expected duplicate item not to be added")
	}

	var count int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM cart_items
		WHERE user_id = $1 AND menu_item_id = $2
	`, userID, menuItemID).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expected 1 cart item, got %d", count)
	}
}

func TestGetItems(t *testing.T) {
	db := setupCartTestDB(t)
	defer db.Close()

	userID := createCartUser(t, db)
	menuItemID := createMenuItem(t, db)

	_, err := AddItem(db, userID, menuItemID)
	if err != nil {
		t.Fatal(err)
	}

	items, err := GetItems(db, userID)

	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 cart item, got %d", len(items))
	}

	item := items[0]
	if item.MenuItemID != menuItemID {
		t.Fatalf("expected menu item ID %d, got %d", menuItemID, item.MenuItemID)
	}

	if item.Name != "Jollof Rice" {
		t.Fatalf("expected Jollof Rice, got %q", item.Name)
	}

	if item.Quantity != 1 {
		t.Fatalf("expected quantity 1, got %d", item.Quantity)
	}

	if item.PriceKobo != 350000 {
		t.Fatalf("expected price 350000, got %d", item.PriceKobo)
	}
}

func TestRemoveItem(t *testing.T) {
	db := setupCartTestDB(t)
	defer db.Close()

	userID := createCartUser(t, db)
	menuItemID := createMenuItem(t, db)

	_, err := AddItem(db, userID, menuItemID)
	if err != nil {
		t.Fatal(err)
	}

	err = RemoveItem(db, userID, menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	var count int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM cart_items
		WHERE user_id = $1 AND menu_item_id = $2
	`, userID, menuItemID).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("expected cart item to be removed, got %d", count)
	}
}

func TestClearCart(t *testing.T) {
	db := setupCartTestDB(t)
	defer db.Close()

	userID := createCartUser(t, db)
	menuItemID := createMenuItem(t, db)

	_, err := AddItem(db, userID, menuItemID)
	if err != nil {
		t.Fatal(err)
	}

	err = ClearCart(db, userID)

	if err != nil {
		t.Fatal(err)
	}

	var count int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM cart_items
		WHERE user_id = $1
	`, userID).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("expected cart to be empty, got %d items", count)
	}
}
