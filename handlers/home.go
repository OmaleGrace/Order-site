package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

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

type homePublicData struct {
	Popular    []HomeDish
	Categories []HomeCategory
	Shops      []ShopCard
	ExpiresAt  time.Time
}

var homeCache struct {
	sync.RWMutex
	data homePublicData
}

var homeTemplate = template.Must(
	template.New("home.html").
		Funcs(template.FuncMap{"naira": naira}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/home.html"),
)

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

func (h *Handlers) loadHomePublicData() homePublicData {
	homeCache.RLock()
	cached := homeCache.data
	homeCache.RUnlock()
	if time.Now().Before(cached.ExpiresAt) {
		return cached
	}

	var (
		popular    []HomeDish
		categories []HomeCategory
		shops      []ShopCard
	)
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		popular = queryDishes(h.DB, `
            SELECT m.id, m.name, m.price_kobo, COALESCE(m.image_url, '')
            FROM menu_items m
            JOIN vendors v ON v.id = m.vendor_id AND v.status = 'active' AND v.paid_until > NOW()
            LEFT JOIN (
                SELECT oi.menu_item_id, SUM(oi.quantity) AS ordered_qty
                FROM order_items oi
                JOIN orders o ON o.id = oi.order_id
                WHERE o.status NOT IN ('pending', 'cancelled')
                GROUP BY oi.menu_item_id
            ) p ON p.menu_item_id = m.id
            WHERE COALESCE(m.available, TRUE) = TRUE
            ORDER BY COALESCE(p.ordered_qty, 0) DESC, m.name
            LIMIT 8
        `)
	}()

	go func() {
		defer wg.Done()
		rows, err := h.DB.Query(`
            SELECT DISTINCT m.category
            FROM menu_items m
            JOIN vendors v ON v.id = m.vendor_id AND v.status = 'active' AND v.paid_until > NOW()
            WHERE COALESCE(m.available, TRUE) = TRUE AND m.category <> ''
            ORDER BY m.category
        `)
		if err != nil {
			log.Printf("home: categories query failed: %v", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil {
				categories = append(categories, HomeCategory{
					Name: name,
					Icon: categoryIcon(name),
					Link: "/menu?category=" + url.QueryEscape(name),
				})
			}
		}
	}()

	go func() {
		defer wg.Done()
		shops = h.topShops(8)
	}()

	wg.Wait()

	fresh := homePublicData{
		Popular:    popular,
		Categories: categories,
		Shops:      shops,
		ExpiresAt:  time.Now().Add(30 * time.Second),
	}
	homeCache.Lock()
	homeCache.data = fresh
	homeCache.Unlock()
	return fresh
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	public := h.loadHomePublicData()

	// Personalized order-again is intentionally kept out of the public cache.
	// It is only one small query and only runs for logged-in customers.
	var again []HomeDish
	if cookie, err := r.Cookie("session_token"); err == nil && cookie.Value != "" {
		again = queryDishes(h.DB, `
            SELECT m.id, m.name, m.price_kobo, COALESCE(m.image_url, '')
            FROM menu_items m
            JOIN vendors v ON v.id = m.vendor_id AND v.status = 'active' AND v.paid_until > NOW()
            JOIN (
                SELECT oi.menu_item_id, MAX(o.created_at) AS last_ordered
                FROM order_items oi
                JOIN orders o ON o.id = oi.order_id
                JOIN sessions s ON s.user_id = o.user_id
                WHERE COALESCE(m.available, TRUE) = TRUE AND s.token = $1 AND s.expires_at > NOW()
                  AND o.status NOT IN ('pending', 'cancelled')
                GROUP BY oi.menu_item_id
            ) r ON r.menu_item_id = m.id
            ORDER BY r.last_ordered DESC
            LIMIT 8
        `, middleware.HashToken(cookie.Value))
	}

	data := struct {
		Popular           []HomeDish
		Again             []HomeDish
		Categories        []HomeCategory
		Shops             []ShopCard
		SetupFeeKobo      int
		CommissionPercent int
	}{
		Popular:           public.Popular,
		Again:             again,
		Categories:        public.Categories,
		Shops:             public.Shops,
		SetupFeeKobo:      vendorSetupFeeKobo(),
		CommissionPercent: commissionPercent(),
	}

	if err := homeTemplate.Execute(w, data); err != nil {
		log.Printf("home: template failed: %v", err)
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
	}
}
