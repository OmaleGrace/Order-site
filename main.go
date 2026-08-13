package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"Order-site/database"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/home.html"))

	err := tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Something went Wrong", http.StatusInternalServerError)
		return
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/login.html"))

	err := tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func signupHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	tmpl := template.Must(template.ParseFiles("templates/signup.html"))

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Something went wrong", http.StatusBadRequest)
			return
		}
	}
	err := tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static", http.StripPrefix("/static/", fs))

	db, err := database.Connect()
	if err != nil
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/signup",func(w http.ResponseWriter, r *http.Request) {
		signupHandler(w, r, db)
	})

	fmt.Println("Server Running on https://localhost:8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}
