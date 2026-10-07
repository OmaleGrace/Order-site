package handlers

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Order-site/errors"
)

type AdminMenuItem struct {
	ID          int
	Name        string
	Description string
	PriceNaira  string
	ImageURL    string
	Category    string
}

type AdminMenuPage struct {
	Items  []AdminMenuItem
	Notice string
}

func (h *Handlers) AdminMenu(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`
		SELECT id, name, COALESCE(description, ''), price_kobo,
		       COALESCE(image_url, ''), COALESCE(category, '')
		FROM menu_items
		ORDER BY category, name
	`)
	if err != nil {
		log.Printf("admin menu: query failed: %v", err)
		errors.Render(w, "Could not load menu", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	page := AdminMenuPage{Notice: r.URL.Query().Get("msg")}

	for rows.Next() {
		var it AdminMenuItem
		var kobo int
		if err := rows.Scan(&it.ID, &it.Name, &it.Description, &kobo, &it.ImageURL, &it.Category); err != nil {
			log.Printf("admin menu: scan failed: %v", err)
			errors.Render(w, "Could not read menu", http.StatusInternalServerError)
			return
		}
		it.PriceNaira = fmt.Sprintf("%.2f", float64(kobo)/100)
		page.Items = append(page.Items, it)
	}
	if err := rows.Err(); err != nil {
		log.Printf("admin menu: rows failed: %v", err)
		errors.Render(w, "Could not read menu", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("templates/admin-menu.html")
	if err != nil {
		log.Printf("admin menu: template failed: %v", err)
		errors.Render(w, "Could not display menu", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("admin menu: execute failed: %v", err)
	}
}

func (h *Handlers) AdminMenuSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}

	idStr := strings.TrimSpace(r.FormValue("id"))
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	imageURL := strings.TrimSpace(r.FormValue("image_url"))
	category := strings.TrimSpace(r.FormValue("category"))

	if name == "" {
		errors.Render(w, "Item name is required", http.StatusBadRequest)
		return
	}

	naira, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("price")), 64)
	if err != nil || naira <= 0 || naira > 10000000 {
		errors.Render(w, "Enter a valid price in naira", http.StatusBadRequest)
		return
	}
	priceKobo := int(math.Round(naira * 100))

	if idStr == "" {
		_, err = h.DB.Exec(`
			INSERT INTO menu_items (name, description, price_kobo, image_url, category)
			VALUES ($1, $2, $3, $4, $5)
		`, name, description, priceKobo, imageURL, category)
	} else {
		id, convErr := strconv.Atoi(idStr)
		if convErr != nil {
			errors.Render(w, "Invalid item", http.StatusBadRequest)
			return
		}
		_, err = h.DB.Exec(`
			UPDATE menu_items
			SET name = $1, description = $2, price_kobo = $3, image_url = $4, category = $5
			WHERE id = $6
		`, name, description, priceKobo, imageURL, category, id)
	}
	if err != nil {
		log.Printf("admin menu: save failed: %v", err)
		errors.Render(w, "Could not save the item", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/menu?msg="+url.QueryEscape("Item saved"), http.StatusSeeOther)
}

func (h *Handlers) AdminMenuDelete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		errors.Render(w, "Invalid item", http.StatusBadRequest)
		return
	}

	if _, err := h.DB.Exec("DELETE FROM menu_items WHERE id = $1", id); err != nil {
		// Most likely the item appears in past orders or carts, so it can't be removed
		log.Printf("admin menu: delete failed: %v", err)
		http.Redirect(w, r, "/admin/menu?msg="+url.QueryEscape("This item has past orders or is in a cart, so it can't be deleted"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/menu?msg="+url.QueryEscape("Item deleted"), http.StatusSeeOther)
}