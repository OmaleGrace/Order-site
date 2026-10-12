package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
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
	Items      []AdminMenuItem
	Notice     string
	Warning    string
	Base       string
	IsAdmin    bool
	VendorName string
}

// ownedVendor finds the vendor that the logged-in customer owns.
func (h *Handlers) ownedVendor(w http.ResponseWriter, r *http.Request) (int, string, bool) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, "", false
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, "", false
	}

	var id int
	var name string
	err = h.DB.QueryRow(
		"SELECT id, name FROM vendors WHERE owner_user_id = $1 ORDER BY id LIMIT 1",
		userID,
	).Scan(&id, &name)
	if err == sql.ErrNoRows {
		errors.Render(w, "Access denied", http.StatusForbidden)
		return 0, "", false
	}
	if err != nil {
		log.Printf("vendor: owner lookup failed: %v", err)
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return 0, "", false
	}
	return id, name, true
}

func menuBase(isAdmin bool) string {
	if isAdmin {
		return "/admin"
	}
	return "/vendor"
}

func menuRedirect(w http.ResponseWriter, r *http.Request, isAdmin bool, msg string) {
	http.Redirect(w, r, menuBase(isAdmin)+"/menu?msg="+url.QueryEscape(msg), http.StatusSeeOther)
}

func (h *Handlers) renderMenuManager(w http.ResponseWriter, r *http.Request, vendorID int, vendorName string, isAdmin bool) {
	query := `
		SELECT id, name, COALESCE(description, ''), price_kobo,
		       COALESCE(image_url, ''), COALESCE(category, '')
		FROM menu_items
	`
	var args []interface{}
	if !isAdmin {
		query += " WHERE vendor_id = $1"
		args = append(args, vendorID)
	}
	query += " ORDER BY category, name"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		log.Printf("menu manager: query failed: %v", err)
		errors.Render(w, "Could not load menu", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	page := AdminMenuPage{
		Notice:     r.URL.Query().Get("msg"),
		Base:       menuBase(isAdmin),
		IsAdmin:    isAdmin,
		VendorName: vendorName,
	}

	for rows.Next() {
		var it AdminMenuItem
		var kobo int
		if err := rows.Scan(&it.ID, &it.Name, &it.Description, &kobo, &it.ImageURL, &it.Category); err != nil {
			log.Printf("menu manager: scan failed: %v", err)
			errors.Render(w, "Could not read menu", http.StatusInternalServerError)
			return
		}
		it.PriceNaira = fmt.Sprintf("%.2f", float64(kobo)/100)
		page.Items = append(page.Items, it)
	}
	if err := rows.Err(); err != nil {
		log.Printf("menu manager: rows failed: %v", err)
		errors.Render(w, "Could not read menu", http.StatusInternalServerError)
		return
	}

	if !isAdmin {
		var live bool
		err := h.DB.QueryRow(
			"SELECT COALESCE(status = 'active' AND paid_until > NOW(), false) FROM vendors WHERE id = $1",
			vendorID,
		).Scan(&live)
		if err != nil {
			log.Printf("menu manager: vendor status failed: %v", err)
		} else if !live {
			page.Warning = "Your shop is not live yet, so customers can't see your dishes. It goes live once it is activated and your payment is recorded."
		}
	}

	tmpl, err := brand.ParseE("templates/admin-menu.html")
	if err != nil {
		log.Printf("menu manager: template failed: %v", err)
		errors.Render(w, "Could not display menu", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("menu manager: execute failed: %v", err)
	}
}

func (h *Handlers) saveMenuItem(w http.ResponseWriter, r *http.Request, vendorID int, isAdmin bool) {
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
		if isAdmin {
			_, err = h.DB.Exec(`
				INSERT INTO menu_items (name, description, price_kobo, image_url, category)
				VALUES ($1, $2, $3, $4, $5)
			`, name, description, priceKobo, imageURL, category)
		} else {
			_, err = h.DB.Exec(`
				INSERT INTO menu_items (name, description, price_kobo, image_url, category, vendor_id)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, name, description, priceKobo, imageURL, category, vendorID)
		}
	} else {
		id, convErr := strconv.Atoi(idStr)
		if convErr != nil {
			errors.Render(w, "Invalid item", http.StatusBadRequest)
			return
		}
		var res sql.Result
		if isAdmin {
			res, err = h.DB.Exec(`
				UPDATE menu_items
				SET name = $1, description = $2, price_kobo = $3, image_url = $4, category = $5
				WHERE id = $6
			`, name, description, priceKobo, imageURL, category, id)
		} else {
			res, err = h.DB.Exec(`
				UPDATE menu_items
				SET name = $1, description = $2, price_kobo = $3, image_url = $4, category = $5
				WHERE id = $6 AND vendor_id = $7
			`, name, description, priceKobo, imageURL, category, id, vendorID)
		}
		if err == nil {
			if n, _ := res.RowsAffected(); n == 0 {
				menuRedirect(w, r, isAdmin, "Item not found")
				return
			}
		}
	}
	if err != nil {
		log.Printf("menu manager: save failed: %v", err)
		errors.Render(w, "Could not save the item", http.StatusInternalServerError)
		return
	}

	menuRedirect(w, r, isAdmin, "Item saved")
}

func (h *Handlers) deleteMenuItem(w http.ResponseWriter, r *http.Request, vendorID int, isAdmin bool) {
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		errors.Render(w, "Invalid item", http.StatusBadRequest)
		return
	}

	var res sql.Result
	if isAdmin {
		res, err = h.DB.Exec("DELETE FROM menu_items WHERE id = $1", id)
	} else {
		res, err = h.DB.Exec("DELETE FROM menu_items WHERE id = $1 AND vendor_id = $2", id, vendorID)
	}
	if err != nil {
		// Most likely the item appears in past orders or carts, so it can't be removed
		log.Printf("menu manager: delete failed: %v", err)
		menuRedirect(w, r, isAdmin, "This item has past orders or is in a cart, so it can't be deleted")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		menuRedirect(w, r, isAdmin, "Item not found")
		return
	}

	menuRedirect(w, r, isAdmin, "Item deleted")
}

// Admin (super admin) versions: see and manage every dish.
func (h *Handlers) AdminMenu(w http.ResponseWriter, r *http.Request) {
	h.renderMenuManager(w, r, 0, "", true)
}
func (h *Handlers) AdminMenuSave(w http.ResponseWriter, r *http.Request) {
	h.saveMenuItem(w, r, 0, true)
}
func (h *Handlers) AdminMenuDelete(w http.ResponseWriter, r *http.Request) {
	h.deleteMenuItem(w, r, 0, true)
}

// Vendor owner versions: limited to the vendor they own.
func (h *Handlers) VendorMenu(w http.ResponseWriter, r *http.Request) {
	id, name, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	h.renderMenuManager(w, r, id, name, false)
}
func (h *Handlers) VendorMenuSave(w http.ResponseWriter, r *http.Request) {
	id, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	h.saveMenuItem(w, r, id, false)
}
func (h *Handlers) VendorMenuDelete(w http.ResponseWriter, r *http.Request) {
	id, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	h.deleteMenuItem(w, r, id, false)
}
