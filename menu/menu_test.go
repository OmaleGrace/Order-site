package menu

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

func setupMenuTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open(
		"postgres",
		"postgresql://testuser:testpass@localhost:5433/food_ordering_test?sslmode=disable",
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		DROP TABLE IF EXISTS menu_items CASCADE;

		CREATE TABLE menu_items (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			price_kobo INTEGER NOT NULL,
			image_url TEXT,
			category TEXT
		);
	`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	return db
}
func TestGetAll(t *testing.T) {
	db := setupMenuTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO menu_items
			(name, description, price_kobo, image_url, category)
		VALUES
			('Jollof Rice', 'Delicious jollof rice', 350000,
			 'https://example.com/jollof.jpg', 'Rice & Swallow'),
			('Fried Rice', 'Tasty fried rice', 300000, NULL, 'Rice & Swallow')
	`)
	if err != nil {
		t.Fatal(err)
	}

	items, err := GetAll(db, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 menu items, got %d", len(items))
	}

	if items[0].Name != "Jollof Rice" {
		t.Fatalf("expected Jollof Rice, got %q", items[0].Name)
	}

	if items[0].PriceKobo != 350000 {
		t.Fatalf("expected price 350000, got %d", items[0].PriceKobo)
	}

	if !items[0].ImageURL.Valid {
		t.Fatal("expected first item to have an image URL")
	}

	if items[0].ImageURL.String != "https://example.com/jollof.jpg" {
		t.Fatalf("unexpected image URL: %q", items[0].ImageURL.String)
	}

	if items[1].Name != "Fried Rice" {
		t.Fatalf("expected Fried Rice, got %q", items[1].Name)
	}

	if items[1].ImageURL.Valid {
		t.Fatal("expected second item image URL to be NULL")
	}
}

func TestGetAllReturnsItemsInIDOrder(t *testing.T) {
	db := setupMenuTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO menu_items
			(name, description, price_kobo)
		VALUES
			('First Item', 'First', 100000),
			('Second Item', 'Second', 200000),
			('Third Item', 'Third', 300000)
	`)
	if err != nil {
		t.Fatal(err)
	}

	items, err := GetAll(db, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 menu items, got %d", len(items))
	}

	if items[0].Name != "First Item" {
		t.Fatalf("expected First Item, got %q", items[0].Name)
	}

	if items[1].Name != "Second Item" {
		t.Fatalf("expected Second Item, got %q", items[1].Name)
	}

	if items[2].Name != "Third Item" {
		t.Fatalf("expected Third Item, got %q", items[2].Name)
	}
}

func TestGetAllFiltersBySearch(t *testing.T) {
	db := setupMenuTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO menu_items
			(name, description, price_kobo, category)
		VALUES
			('Jollof Rice', 'Delicious jollof rice', 350000, 'Rice & Swallow'),
			('Fried Rice', 'Tasty fried rice', 300000, 'Rice & Swallow'),
			('Chapman', 'Classic non-alcoholic cocktail', 120000, 'Drinks')
	`)
	if err != nil {
		t.Fatal(err)
	}

	items, err := GetAll(db, "rice", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 menu items matching 'rice', got %d", len(items))
	}

	for _, item := range items {
		if item.Name != "Jollof Rice" && item.Name != "Fried Rice" {
			t.Fatalf("unexpected item in search results: %q", item.Name)
		}
	}
}

func TestGetAllFiltersByCategory(t *testing.T) {
	db := setupMenuTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO menu_items
			(name, description, price_kobo, category)
		VALUES
			('Jollof Rice', 'Delicious jollof rice', 350000, 'Rice & Swallow'),
			('Chapman', 'Classic non-alcoholic cocktail', 120000, 'Drinks'),
			('Zobo', 'Chilled hibiscus drink', 80000, 'Drinks')
	`)
	if err != nil {
		t.Fatal(err)
	}

	items, err := GetAll(db, "", "Drinks")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 menu items in Drinks category, got %d", len(items))
	}

	for _, item := range items {
		if !item.Category.Valid || item.Category.String != "Drinks" {
			t.Fatalf("unexpected category for item %q: %v", item.Name, item.Category)
		}
	}
}

func TestGetAllFiltersBySearchAndCategory(t *testing.T) {
	db := setupMenuTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO menu_items
			(name, description, price_kobo, category)
		VALUES
			('Jollof Rice', 'Delicious jollof rice', 350000, 'Rice & Swallow'),
			('Fried Rice', 'Tasty fried rice', 300000, 'Rice & Swallow'),
			('Chapman', 'Rice-free classic cocktail', 120000, 'Drinks')
	`)
	if err != nil {
		t.Fatal(err)
	}

	items, err := GetAll(db, "rice", "Rice & Swallow")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 menu items, got %d", len(items))
	}

	for _, item := range items {
		if !item.Category.Valid || item.Category.String != "Rice & Swallow" {
			t.Fatalf("unexpected category for item %q: %v", item.Name, item.Category)
		}
	}
}