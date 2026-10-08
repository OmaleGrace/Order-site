package handlers

import (
	"fmt"
	"net/http"

	"Order-site/brand"
	"Order-site/menu"
	"Order-site/middleware"

	"html/template"
)

func naira(kobo int) int {
	return kobo / 100
}

func (h *Handlers) Menu(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(
		template.New("menu.html").
			Funcs(template.FuncMap{
				"naira": naira,
			}).
			Funcs(brand.Funcs()).
			ParseFiles("templates/menu.html"),
	)

	search := r.URL.Query().Get("search")
	category := r.URL.Query().Get("category")

	items, err := menu.GetAll(h.DB, search, category)
	if err != nil {
		fmt.Println("Menu load error", err)
		http.Error(w, "Could not load menu", http.StatusInternalServerError)
		return
	}

	categories := []string{}
	catRows, err := h.DB.Query(`
		SELECT DISTINCT category FROM menu_items
		WHERE category <> '' ORDER BY category
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
	if cookie, err := r.Cookie("session_token"); err == nil {
		if userID, err := middleware.GetUserID(h.DB, cookie.Value); err == nil {
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
	}{
		Items:      items,
		Message:    message,
		Search:     search,
		Category:   category,
		Categories: categories,
		Favs:       favs,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Template error:", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}