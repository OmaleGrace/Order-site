package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
)

type AdminRider struct {
	ID        int
	Name      string
	Phone     string
	Vehicle   string
	Active    bool
	Assigned  int
	Completed int
}

type AdminRidersPage struct {
	Riders []AdminRider
	Notice string
}

func (h *Handlers) AdminRiders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := h.DB.Query(`
        SELECT r.id, r.name, r.phone, r.vehicle, r.active,
               (SELECT COUNT(*) FROM orders o WHERE o.rider_id=r.id AND o.status IN ('paid','preparing','ready') AND o.delivery_method='delivery'),
               (SELECT COUNT(*) FROM orders o WHERE o.rider_id=r.id AND o.status='completed' AND o.delivery_method='delivery')
        FROM riders r ORDER BY r.active DESC, r.name ASC
    `)
	if err != nil {
		log.Printf("admin riders: query failed: %v", err)
		errors.Render(w, "Could not load riders", 500)
		return
	}
	defer rows.Close()
	page := AdminRidersPage{Notice: r.URL.Query().Get("msg")}
	for rows.Next() {
		var rider AdminRider
		if err := rows.Scan(&rider.ID, &rider.Name, &rider.Phone, &rider.Vehicle, &rider.Active, &rider.Assigned, &rider.Completed); err != nil {
			errors.Render(w, "Could not read riders", 500)
			return
		}
		page.Riders = append(page.Riders, rider)
	}
	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read riders", 500)
		return
	}
	tmpl, err := brand.ParseE("templates/admin-riders.html")
	if err != nil {
		errors.Render(w, "Could not display riders", 500)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("admin riders: execute failed: %v", err)
	}
}

func (h *Handlers) AdminRiderSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Invalid form", 400)
		return
	}
	id, _ := strconv.Atoi(r.FormValue("id"))
	name := strings.TrimSpace(r.FormValue("name"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	vehicle := strings.TrimSpace(r.FormValue("vehicle"))
	if len([]rune(name)) < 2 || len([]rune(name)) > 100 {
		http.Redirect(w, r, "/admin/riders?msg=Enter+a+valid+rider+name", 303)
		return
	}
	if len([]rune(phone)) < 7 || len([]rune(phone)) > 30 {
		http.Redirect(w, r, "/admin/riders?msg=Enter+a+valid+phone+number", 303)
		return
	}
	if id > 0 {
		if _, err := h.DB.Exec(`UPDATE riders SET name=$1,phone=$2,vehicle=$3,active=$4 WHERE id=$5`, name, phone, vehicle, r.FormValue("active") == "on", id); err != nil {
			log.Printf("admin riders: update failed: %v", err)
			http.Redirect(w, r, "/admin/riders?msg=Could+not+update+rider", 303)
			return
		}
	} else {
		if _, err := h.DB.Exec(`INSERT INTO riders(name,phone,vehicle,active) VALUES($1,$2,$3,TRUE)`, name, phone, vehicle); err != nil {
			log.Printf("admin riders: insert failed: %v", err)
			http.Redirect(w, r, "/admin/riders?msg=Could+not+add+rider", 303)
			return
		}
	}
	http.Redirect(w, r, "/admin/riders?msg=Rider+saved", 303)
}

func (h *Handlers) AdminRiderToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", 405)
		return
	}
	id, _ := strconv.Atoi(r.FormValue("id"))
	if id < 1 {
		http.Redirect(w, r, "/admin/riders?msg=Invalid+rider", 303)
		return
	}
	if _, err := h.DB.Exec(`UPDATE riders SET active=NOT active WHERE id=$1`, id); err != nil {
		log.Printf("admin riders: toggle failed: %v", err)
		http.Redirect(w, r, "/admin/riders?msg=Could+not+update+rider", 303)
		return
	}
	http.Redirect(w, r, "/admin/riders?msg=Rider+status+updated", 303)
}
