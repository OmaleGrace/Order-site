package errors

import (
	"log"
	"net/http"

	"Order-site/render"
)

var titles = map[int]string{
	http.StatusBadRequest:          "Something's Not Right",
	http.StatusUnauthorized:        "You Need to Log In",
	http.StatusForbidden:           "You Don't Have Access",
	http.StatusNotFound:            "Page Not Found",
	http.StatusMethodNotAllowed:    "That Didn't Work",
	http.StatusTooManyRequests:     "Please Slow Down",
	http.StatusInternalServerError: "Something Went Wrong",
}

// Render shows the site's branded error page with a message the visitor can act on.
func Render(w http.ResponseWriter, message string, code int) {
	title, ok := titles[code]
	if !ok {
		title = "Something Went Wrong"
	}

	tmpl, err := render.Page("error.html")
	if err != nil {
		log.Printf("error page template: %v", err)
		http.Error(w, message, code)
		return
	}

	data := struct {
		Code       int
		Title      string
		Message    string
		NeedsLogin bool // offer a log-in button
		CanRetry   bool // the problem is on our side, so trying again may work
	}{
		Code:       code,
		Title:      title,
		Message:    message,
		NeedsLogin: code == http.StatusUnauthorized,
		CanRetry:   code >= 500 || code == http.StatusTooManyRequests,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("error page render: %v", err)
	}
}
