package handlers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"Order-site/database"
)

func serveMedia(t *testing.T, h *Handlers, id int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(id, 10), nil)
	req.SetPathValue("id", strconv.FormatInt(id, 10))
	w := httptest.NewRecorder()
	h.ServeUploadedImage(w, req)
	return w
}

func TestServeUploadedImageFromDatabaseAndCloud(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	h := New(db)

	var stored, moved, freed int64
	db.QueryRow(`INSERT INTO uploaded_images (content_type, data) VALUES ('image/png', '\x89504e47') RETURNING id`).Scan(&stored)
	db.QueryRow(`INSERT INTO uploaded_images (content_type, data, cloud_url) VALUES ('image/png', '\x89504e47', 'https://res.cloudinary.com/demo/a.png') RETURNING id`).Scan(&moved)
	db.QueryRow(`INSERT INTO uploaded_images (content_type, data, cloud_url) VALUES ('image/png', NULL, 'https://res.cloudinary.com/demo/b.png') RETURNING id`).Scan(&freed)

	if w := serveMedia(t, h, stored); w.Code != http.StatusOK || w.Body.Len() == 0 {
		t.Fatalf("a picture kept in the database should be served directly, got %d", w.Code)
	}
	for id, want := range map[int64]string{moved: "https://res.cloudinary.com/demo/a.png", freed: "https://res.cloudinary.com/demo/b.png"} {
		w := serveMedia(t, h, id)
		if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != want {
			t.Fatalf("picture %d should forward to Cloudinary, got %d %q", id, w.Code, w.Header().Get("Location"))
		}
	}
	if w := serveMedia(t, h, 999999); w.Code != http.StatusNotFound {
		t.Fatalf("a missing picture should be 404, got %d", w.Code)
	}
	// A row whose bytes were emptied but that has no Cloudinary address must not serve an empty file.
	var broken int64
	db.QueryRow(`INSERT INTO uploaded_images (content_type, data) VALUES ('image/png', NULL) RETURNING id`).Scan(&broken)
	if w := serveMedia(t, h, broken); w.Code != http.StatusNotFound {
		t.Fatalf("an empty picture should be 404, got %d", w.Code)
	}
}

// Cleanup removes pictures nobody uses, but must keep ones used by a dish, a shop logo or a review.
func TestCleanupUnusedImagesKeepsReviewPhotos(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ids := map[string]int64{}
	for _, name := range []string{"dish", "logo", "review", "orphan"} {
		var id int64
		if err := db.QueryRow(`INSERT INTO uploaded_images (content_type, data, created_at) VALUES ('image/png', '\x89504e47', NOW() - INTERVAL '3 days') RETURNING id`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	ref := func(id int64) string { return "/media/" + strconv.FormatInt(id, 10) }
	var userID, orderID int
	if err := db.QueryRow(`INSERT INTO users (name, email, password) VALUES ('R', 'r@example.com', 'x') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO orders (user_id, vendor_id, total_kobo, status) VALUES ($1, 1, 1000, 'completed') RETURNING id`, userID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO menu_items (vendor_id, name, description, price_kobo, image_url) VALUES (1, 'Jollof', 'd', 1000, $1)`, ref(ids["dish"])); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE vendors SET logo_url = $1 WHERE id = 1`, ref(ids["logo"])); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_reviews (order_id, user_id, vendor_id, rating, comment, photo_url) VALUES ($1, $2, 1, 5, 'great', $3)`, orderID, userID, ref(ids["review"])); err != nil {
		t.Fatal(err)
	}

	if err := database.CleanupUnusedImages(db); err != nil {
		t.Fatal(err)
	}
	for name, id := range ids {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM uploaded_images WHERE id = $1`, id).Scan(&n)
		if name == "orphan" && n != 0 {
			t.Errorf("the unused picture should have been removed")
		}
		if name != "orphan" && n != 1 {
			t.Errorf("the %s picture is in use and must be kept", name)
		}
	}
}
