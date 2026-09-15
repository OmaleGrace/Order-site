package handlers

import (
	"Order-site/errors"
	"html/template"
	"net/http"
)

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/home.html"))

	err := tmpl.Execute(w, nil)
	if err != nil {
		errors.Render(w, "Something went Wrong", http.StatusInternalServerError)
		return
	}
}
