package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"Order-site/errors"
	"Order-site/render"
)

type CustomOption struct {
	ID        int    `json:"id"`
	GroupID   int    `json:"group_id"`
	Group     string `json:"group"`
	Name      string `json:"name"`
	PriceKobo int    `json:"price_kobo"`
}

type CustomGroup struct {
	ID        int            `json:"id"`
	Name      string         `json:"name"`
	MinSelect int            `json:"min_select"`
	MaxSelect int            `json:"max_select"`
	Options   []CustomOption `json:"options"`
}

// MenuItemOptions returns the currently available customization choices for a dish.
func (h *Handlers) MenuItemOptions(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid item", http.StatusBadRequest)
		return
	}
	rows, err := h.DB.Query(`
        SELECT g.id, g.name, g.min_select, g.max_select, o.id, o.name, o.price_kobo
        FROM menu_option_groups g
        JOIN menu_options o ON o.group_id = g.id
        WHERE g.menu_item_id = $1 AND g.active = TRUE AND o.active = TRUE
        ORDER BY g.position, g.id, o.position, o.id
    `, id)
	if err != nil {
		log.Printf("menu options: query failed: %v", err)
		http.Error(w, "could not load options", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	groups := []CustomGroup{}
	index := map[int]int{}
	for rows.Next() {
		var g CustomGroup
		var o CustomOption
		if err := rows.Scan(&g.ID, &g.Name, &g.MinSelect, &g.MaxSelect, &o.ID, &o.Name, &o.PriceKobo); err != nil {
			http.Error(w, "could not load options", http.StatusInternalServerError)
			return
		}
		o.GroupID, o.Group = g.ID, g.Name
		if i, ok := index[g.ID]; ok {
			groups[i].Options = append(groups[i].Options, o)
		} else {
			g.Options = []CustomOption{o}
			index[g.ID] = len(groups)
			groups = append(groups, g)
		}
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "could not load options", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(groups)
}

func parseOptionIDs(values []string) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(part))
			if err == nil && id > 0 && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// VendorOptionGroups returns the customization configuration for the vendor menu manager.
func (h *Handlers) VendorOptionGroups(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	itemID, err := strconv.Atoi(r.URL.Query().Get("item_id"))
	if err != nil {
		errors.Render(w, "Invalid item", 400)
		return
	}
	var owns bool
	if err := h.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM menu_items WHERE id=$1 AND vendor_id=$2)`, itemID, vendorID).Scan(&owns); err != nil || !owns {
		errors.Render(w, "Item not found", 404)
		return
	}
	rows, err := h.DB.Query(`SELECT g.id,g.name,g.min_select,g.max_select,g.active,o.id,o.name,o.price_kobo,o.active FROM menu_option_groups g LEFT JOIN menu_options o ON o.group_id=g.id WHERE g.menu_item_id=$1 ORDER BY g.position,g.id,o.position,o.id`, itemID)
	if err != nil {
		errors.Render(w, "Could not load customizations", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		GroupID      int
		GroupName    string
		Min          int
		Max          int
		GroupActive  bool
		OptionID     int
		OptionName   string
		Price        int
		OptionActive bool
	}
	var out []Row
	for rows.Next() {
		var x Row
		var oid sql.NullInt64
		var oname sql.NullString
		var price sql.NullInt64
		var active sql.NullBool
		if err := rows.Scan(&x.GroupID, &x.GroupName, &x.Min, &x.Max, &x.GroupActive, &oid, &oname, &price, &active); err != nil {
			errors.Render(w, "Could not read customizations", 500)
			return
		}
		if oid.Valid {
			x.OptionID = int(oid.Int64)
			x.OptionName = oname.String
			x.Price = int(price.Int64)
			x.OptionActive = active.Bool
		}
		out = append(out, x)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *Handlers) VendorCustomizationPage(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	itemID, err := strconv.Atoi(r.URL.Query().Get("item_id"))
	if err != nil || itemID <= 0 {
		errors.Render(w, "Invalid item", 400)
		return
	}
	var itemName string
	if err := h.DB.QueryRow(`SELECT name FROM menu_items WHERE id=$1 AND vendor_id=$2`, itemID, vendorID).Scan(&itemName); err != nil {
		errors.Render(w, "Item not found", 404)
		return
	}
	rows, err := h.DB.Query(`SELECT g.id,g.name,g.min_select,g.max_select,g.active,COALESCE(o.id,0),COALESCE(o.name,''),COALESCE(o.price_kobo,0),COALESCE(o.active,false) FROM menu_option_groups g LEFT JOIN menu_options o ON o.group_id=g.id WHERE g.menu_item_id=$1 ORDER BY g.position,g.id,o.position,o.id`, itemID)
	if err != nil {
		errors.Render(w, "Could not load customizations", 500)
		return
	}
	defer rows.Close()
	type Opt struct {
		ID     int
		Name   string
		Price  int
		Active bool
	}
	type Group struct {
		ID      int
		Name    string
		Min     int
		Max     int
		Active  bool
		Options []Opt
	}
	groups := []Group{}
	idx := map[int]int{}
	for rows.Next() {
		var g Group
		var oid int
		var on string
		var p int
		var oa bool
		if err := rows.Scan(&g.ID, &g.Name, &g.Min, &g.Max, &g.Active, &oid, &on, &p, &oa); err != nil {
			errors.Render(w, "Could not read customizations", 500)
			return
		}
		if i, ok := idx[g.ID]; ok {
			if oid > 0 {
				groups[i].Options = append(groups[i].Options, Opt{oid, on, p, oa})
			}
		} else {
			idx[g.ID] = len(groups)
			if oid > 0 {
				g.Options = []Opt{{oid, on, p, oa}}
			}
			groups = append(groups, g)
		}
	}
	tmpl, err := render.Page("vendor-customizations.html", template.FuncMap{"naira": naira})
	if err != nil {
		errors.Render(w, "Could not display customizations", 500)
		return
	}
	_ = tmpl.Execute(w, struct {
		VendorName, ItemName string
		ItemID               int
		Groups               []Group
		Notice               string
	}{vendorName, itemName, itemID, groups, r.URL.Query().Get("msg")})
}

func (h *Handlers) VendorCustomizationGroupSave(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	itemID, _ := strconv.Atoi(r.FormValue("item_id"))
	name := strings.TrimSpace(r.FormValue("name"))
	minS, _ := strconv.Atoi(r.FormValue("min_select"))
	maxS, _ := strconv.Atoi(r.FormValue("max_select"))
	id, _ := strconv.Atoi(r.FormValue("group_id"))
	if name == "" || len([]rune(name)) > 80 || minS < 0 || maxS < 1 || minS > maxS {
		errors.Render(w, "Enter valid customization group details", 400)
		return
	}
	var err error
	if id > 0 {
		_, err = h.DB.Exec(`UPDATE menu_option_groups SET name=$1,min_select=$2,max_select=$3 WHERE id=$4 AND menu_item_id=$5 AND EXISTS(SELECT 1 FROM menu_items WHERE id=$5 AND vendor_id=$6)`, name, minS, maxS, id, itemID, vendorID)
	} else {
		_, err = h.DB.Exec(`INSERT INTO menu_option_groups(menu_item_id,name,min_select,max_select) SELECT $1,$2,$3,$4 WHERE EXISTS(SELECT 1 FROM menu_items WHERE id=$1 AND vendor_id=$5)`, itemID, name, minS, maxS, vendorID)
	}
	if err != nil {
		errors.Render(w, "Could not save customization group", 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/vendor/menu/customizations?item_id=%d&msg=Group+saved", itemID), 303)
}

func (h *Handlers) VendorCustomizationOptionSave(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	itemID, _ := strconv.Atoi(r.FormValue("item_id"))
	groupID, _ := strconv.Atoi(r.FormValue("group_id"))
	optionID, _ := strconv.Atoi(r.FormValue("option_id"))
	name := strings.TrimSpace(r.FormValue("name"))
	priceN, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("price")), 64)
	if name == "" || len([]rune(name)) > 80 || priceN < 0 || priceN > 1000000 {
		errors.Render(w, "Enter valid option details", 400)
		return
	}
	price := int(math.Round(priceN * 100))
	var err error
	if optionID > 0 {
		_, err = h.DB.Exec(`UPDATE menu_options SET name=$1,price_kobo=$2 WHERE id=$3 AND group_id=$4 AND EXISTS(SELECT 1 FROM menu_option_groups g JOIN menu_items m ON m.id=g.menu_item_id WHERE g.id=$4 AND m.id=$5 AND m.vendor_id=$6)`, name, price, optionID, groupID, itemID, vendorID)
	} else {
		_, err = h.DB.Exec(`INSERT INTO menu_options(group_id,name,price_kobo) SELECT $1,$2,$3 WHERE EXISTS(SELECT 1 FROM menu_option_groups g JOIN menu_items m ON m.id=g.menu_item_id WHERE g.id=$1 AND m.id=$4 AND m.vendor_id=$5)`, groupID, name, price, itemID, vendorID)
	}
	if err != nil {
		errors.Render(w, "Could not save customization option", 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/vendor/menu/customizations?item_id=%d&msg=Option+saved", itemID), 303)
}

func (h *Handlers) VendorCustomizationToggle(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	itemID, _ := strconv.Atoi(r.FormValue("item_id"))
	groupID, _ := strconv.Atoi(r.FormValue("group_id"))
	optionID, _ := strconv.Atoi(r.FormValue("option_id"))
	var err error
	if optionID > 0 {
		_, err = h.DB.Exec(`UPDATE menu_options o SET active=NOT o.active WHERE o.id=$1 AND o.group_id=$2 AND EXISTS(SELECT 1 FROM menu_option_groups g JOIN menu_items m ON m.id=g.menu_item_id WHERE g.id=$2 AND m.id=$3 AND m.vendor_id=$4)`, optionID, groupID, itemID, vendorID)
	} else {
		_, err = h.DB.Exec(`UPDATE menu_option_groups g SET active=NOT g.active WHERE g.id=$1 AND g.menu_item_id=$2 AND EXISTS(SELECT 1 FROM menu_items WHERE id=$2 AND vendor_id=$3)`, groupID, itemID, vendorID)
	}
	if err != nil {
		errors.Render(w, "Could not update customization", 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/vendor/menu/customizations?item_id=%d", itemID), 303)
}
