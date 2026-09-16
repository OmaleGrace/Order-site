package handlers

import (
	"html/template"
	"net/http"

	"Order-site/menu"
)

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(
		template.New("home.html").
			Funcs(template.FuncMap{
				"naira": naira,
			}).
			ParseFiles("templates/home.html"),
	)

	items, err := menu.GetAll(h.DB, "", "")
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// Show up to 4 items as a preview
	preview := items
	if len(preview) > 4 {
		preview = preview[:4]
	}

	data := struct {
		Preview []menu.MenuItem
	}{
		Preview: preview,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}