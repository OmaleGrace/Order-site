package handlers

import (
	"fmt"
	"html/template"
	"net/http"

	"Order-site/menu"
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
			ParseFiles("templates/menu.html"),
	)

	search := r.URL.Query().Get("search")
	category := r.URL.Query().Get("category")

	items, err := menu.GetAll(h.DB, search, category)
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

	if err != nil {
		fmt.Println("Menu load error", err)
		http.Error(w, "Could not load menu", http.StatusInternalServerError)
		return
	}

	message := r.URL.Query().Get("message")

		data := struct {
		Items      []menu.MenuItem
		Message    string
		Search     string
		Category   string
		Categories []string
	}{
		Items:      items,
		Message:    message,
		Search:     search,
		Category:   category,
		Categories: categories,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Template error:", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}