package errors

import (
	"html/template"
	"net/http"
)

var titles = map[int]string{
	http.StatusBadRequest:          "Something's Not Right",
	http.StatusUnauthorized:        "You Need to Log In",
	http.StatusMethodNotAllowed:    "That Didn't Work",
	http.StatusInternalServerError: "Something Went Wrong",
	http.StatusNotFound:            "Page Not Found",
}

func Render(w http.ResponseWriter, message string, code int) {
	title, ok := titles[code]
	if !ok {
		title = "Something Went Wrong"
	}

	tmpl, err := template.ParseFiles("templates/error.html")
	if err != nil {
		http.Error(w, message, code)
		return
	}

	data := struct {
		Code    int
		Title   string
		Message string
	}{
		Code:    code,
		Title:   title,
		Message: message,
	}

	w.WriteHeader(code)
	tmpl.Execute(w, data)
}