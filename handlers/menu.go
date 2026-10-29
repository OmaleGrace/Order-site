package handlers

import (
	"Order-site/errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"Order-site/menu"
	"Order-site/middleware"
	"Order-site/render"

	"html/template"
)

func naira(kobo int) int {
	return kobo / 100
}

func (h *Handlers) Menu(w http.ResponseWriter, r *http.Request) {
	tmpl := render.MustPage("menu.html", template.FuncMap{
		"naira": naira,
	})

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	sortBy := r.URL.Query().Get("sort")
	if sortBy != "price_asc" && sortBy != "price_desc" && sortBy != "name" {
		sortBy = "relevance"
	}
	minPrice, _ := strconv.Atoi(r.URL.Query().Get("min_price"))
	maxPrice, _ := strconv.Atoi(r.URL.Query().Get("max_price"))
	if minPrice < 0 {
		minPrice = 0
	}
	if maxPrice < 0 {
		maxPrice = 0
	}
	// Values are entered as naira in the UI; the database stores kobo.
	minPrice *= 100
	maxPrice *= 100
	if len([]rune(search)) > 80 {
		search = string([]rune(search)[:80])
	}
	if len([]rune(category)) > 40 {
		category = string([]rune(category)[:40])
	}

	items, err := menu.GetAllWithOptions(h.DB, search, category, sortBy, minPrice, maxPrice)
	if err != nil {
		fmt.Println("Menu load error", err)
		errors.Render(w, "We couldn't load the menu right now. Please try again in a moment.", http.StatusInternalServerError)
		return
	}

	categories := []string{}
	catRows, err := h.DB.Query(`
		SELECT DISTINCT m.category FROM menu_items m JOIN vendors v ON v.id = m.vendor_id AND v.status = 'active' AND v.paid_until > NOW() WHERE COALESCE(m.available, TRUE) = TRUE AND m.category <> '' ORDER BY m.category
	`)
	if err != nil {
		fmt.Println("Menu categories error:", err)
	} else {
		defer catRows.Close()
		for catRows.Next() {
			var c string
			if err := catRows.Scan(&c); err == nil {
				categories = append(categories, c)
			}
		}
	}

	favs := map[int]bool{}
	loggedIn := false
	if cookie, err := r.Cookie("session_token"); err == nil {
		if userID, err := middleware.GetUserID(h.DB, cookie.Value); err == nil {
			loggedIn = true
			favRows, err := h.DB.Query("SELECT menu_item_id FROM favorites WHERE user_id = $1", userID)
			if err != nil {
				fmt.Println("Menu favourites error:", err)
			} else {
				defer favRows.Close()
				for favRows.Next() {
					var id int
					if err := favRows.Scan(&id); err == nil {
						favs[id] = true
					}
				}
			}
		}
	}

	message := r.URL.Query().Get("message")

	data := struct {
		Items      []menu.MenuItem
		Message    string
		Search     string
		Category   string
		Categories []string
		Favs       map[int]bool
		LoggedIn   bool
		Sort       string
		MinPrice   int
		MaxPrice   int
	}{
		Items:      items,
		Message:    message,
		Search:     search,
		Category:   category,
		Categories: categories,
		Favs:       favs,
		LoggedIn:   loggedIn,
		Sort:       sortBy,
		MinPrice:   minPrice / 100,
		MaxPrice:   maxPrice / 100,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Template error:", err)
		errors.Render(w, "We couldn't load the menu categories right now. Please try again in a moment.", http.StatusInternalServerError)
		return
	}
}
