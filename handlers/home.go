package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
)

type HomeDish struct {
	ID        int
	Name      string
	PriceKobo int
	ImageURL  string
}

type HomeCategory struct {
	Name string
	Icon string
	Link string
}

func categoryIcon(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "drink"):
		return "🥤"
	case strings.Contains(n, "rice"):
		return "🍚"
	case strings.Contains(n, "soup"), strings.Contains(n, "swallow"):
		return "🍲"
	case strings.Contains(n, "snack"):
		return "🥟"
	case strings.Contains(n, "grill"), strings.Contains(n, "protein"), strings.Contains(n, "meat"):
		return "🍗"
	case strings.Contains(n, "dessert"), strings.Contains(n, "cake"):
		return "🍰"
	default:
		return "🍽️"
	}
}

func queryDishes(db *sql.DB, query string, args ...interface{}) []HomeDish {
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("home: dishes query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var out []HomeDish
	for rows.Next() {
		var d HomeDish
		if err := rows.Scan(&d.ID, &d.Name, &d.PriceKobo, &d.ImageURL); err != nil {
			log.Printf("home: dishes scan failed: %v", err)
			continue
		}
		out = append(out, d)
	}
	return out
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(
		template.New("home.html").
			Funcs(template.FuncMap{
				"naira": naira,
			}).
			Funcs(brand.Funcs()).
			ParseFiles("templates/home.html"),
	)

	// Popular now: most ordered items (paid orders only), all items if nothing ordered yet
	popular := queryDishes(h.DB, `
		SELECT m.id, m.name, m.price_kobo, COALESCE(m.image_url, '')
		FROM menu_items m
		LEFT JOIN order_items oi ON oi.menu_item_id = m.id
		LEFT JOIN orders o ON o.id = oi.order_id AND o.status NOT IN ('pending', 'cancelled')
		GROUP BY m.id
		ORDER BY COALESCE(SUM(CASE WHEN o.id IS NOT NULL THEN oi.quantity ELSE 0 END), 0) DESC, m.name
		LIMIT 8
	`)

	// Order again: only for a logged-in customer who has ordered before
	var again []HomeDish
	if cookie, err := r.Cookie("session_token"); err == nil {
		if userID, err := middleware.GetUserID(h.DB, cookie.Value); err == nil {
			again = queryDishes(h.DB, `
				SELECT m.id, m.name, m.price_kobo, COALESCE(m.image_url, '')
				FROM menu_items m
				JOIN (
					SELECT oi.menu_item_id, MAX(o.created_at) AS last_ordered
					FROM order_items oi
					JOIN orders o ON o.id = oi.order_id
					WHERE o.user_id = $1 AND o.status NOT IN ('pending', 'cancelled')
					GROUP BY oi.menu_item_id
				) r ON r.menu_item_id = m.id
				ORDER BY r.last_ordered DESC
				LIMIT 8
			`, userID)
		}
	}

	var categories []HomeCategory
	catRows, err := h.DB.Query(`
		SELECT DISTINCT category FROM menu_items
		WHERE category <> '' ORDER BY category
	`)
	if err != nil {
		log.Printf("home: categories query failed: %v", err)
	} else {
		defer catRows.Close()
		for catRows.Next() {
			var name string
			if err := catRows.Scan(&name); err == nil {
				categories = append(categories, HomeCategory{
					Name: name,
					Icon: categoryIcon(name),
					Link: "/menu?category=" + url.QueryEscape(name),
				})
			}
		}
	}

	data := struct {
		Popular    []HomeDish
		Again      []HomeDish
		Categories []HomeCategory
	}{
		Popular:    popular,
		Again:      again,
		Categories: categories,
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("home: template failed: %v", err)
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
	}
}