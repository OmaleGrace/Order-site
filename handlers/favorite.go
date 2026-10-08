package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
)

type FavItem struct {
	ID          int
	Name        string
	Description string
	PriceKobo   int
	ImageURL    string
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ToggleFavorite adds the dish to the customer's favourites, or removes it if already saved.
func (h *Handlers) ToggleFavorite(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie, err := r.Cookie("session_token")
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "login required")
		return
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "login required")
		return
	}

	if err := r.ParseForm(); err != nil {
		jsonError(w, http.StatusBadRequest, "bad request")
		return
	}
	itemID, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid item")
		return
	}

	res, err := h.DB.Exec(
		"DELETE FROM favorites WHERE user_id = $1 AND menu_item_id = $2",
		userID, itemID,
	)
	if err != nil {
		log.Printf("favorites: delete failed: %v", err)
		jsonError(w, http.StatusInternalServerError, "could not update")
		return
	}

	removed, _ := res.RowsAffected()
	favorited := false

	if removed == 0 {
		_, err = h.DB.Exec(`
			INSERT INTO favorites (user_id, menu_item_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, userID, itemID)
		if err != nil {
			log.Printf("favorites: insert failed: %v", err)
			jsonError(w, http.StatusInternalServerError, "could not update")
			return
		}
		favorited = true
	}

	json.NewEncoder(w).Encode(map[string]bool{"favorited": favorited})
}

// Favorites shows the customer's saved dishes.
func (h *Handlers) Favorites(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	rows, err := h.DB.Query(`
		SELECT m.id, m.name, COALESCE(m.description, ''), m.price_kobo, COALESCE(m.image_url, '')
		FROM favorites f
		JOIN menu_items m ON m.id = f.menu_item_id
		WHERE f.user_id = $1
		ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		log.Printf("favorites: query failed: %v", err)
		errors.Render(w, "Could not load your favourites", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []FavItem
	for rows.Next() {
		var it FavItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Description, &it.PriceKobo, &it.ImageURL); err != nil {
			log.Printf("favorites: scan failed: %v", err)
			errors.Render(w, "Could not read your favourites", http.StatusInternalServerError)
			return
		}
		items = append(items, it)
	}

	tmpl, err := template.New("favorites.html").
		Funcs(template.FuncMap{"naira": naira}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/favorites.html")
	if err != nil {
		log.Printf("favorites: template failed: %v", err)
		errors.Render(w, "Could not display favourites", http.StatusInternalServerError)
		return
	}

	data := struct{ Items []FavItem }{Items: items}
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("favorites: execute failed: %v", err)
	}
}