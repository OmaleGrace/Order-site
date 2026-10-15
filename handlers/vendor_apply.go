package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
)

type IDTypeOption struct {
	Value string
	Label string
}

var idTypeOptions = []IDTypeOption{
	{"nin", "NIN slip or card"},
	{"drivers_licence", "Driver's licence"},
	{"voters_card", "Permanent voter's card"},
	{"passport", "International passport"},
}

func validIDType(v string) bool {
	for _, o := range idTypeOptions {
		if o.Value == v {
			return true
		}
	}
	return false
}

type VendorApplyPage struct {
	State          string // form, pending, rejected, owner
	Notice         string
	AdminNote      string
	BusinessName   string
	Phone          string
	SetupFeeKobo   int
	MonthlyFeeKobo int
	IDTypes        []IDTypeOption
}

func (h *Handlers) sessionUserID(r *http.Request) (int, bool) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return 0, false
	}
	id, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		return 0, false
	}
	return id, true
}

func validFullName(name string) string {
	words := strings.Fields(name)
	if len(words) < 2 {
		return "Enter your full legal name, first and last name"
	}
	if len([]rune(name)) > 80 {
		return "Your name must be 80 characters or fewer"
	}
	for _, w := range words {
		if len([]rune(w)) < 2 {
			return "Each part of your name must be at least 2 letters"
		}
		for _, r := range w {
			if !unicode.IsLetter(r) && r != '-' && r != '\'' && r != '’' && r != '.' {
				return "Your name can only contain letters"
			}
		}
	}
	return ""
}

func (h *Handlers) VendorApply(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.sessionUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	page := VendorApplyPage{
		Notice:         r.URL.Query().Get("msg"),
		SetupFeeKobo:   vendorSetupFeeKobo(),
		MonthlyFeeKobo: vendorMonthlyFeeKobo(),
		IDTypes:        idTypeOptions,
	}

	var owned int
	if err := h.DB.QueryRow("SELECT count(*) FROM vendors WHERE owner_user_id = $1", userID).Scan(&owned); err != nil {
		log.Printf("vendor apply: owner check failed: %v", err)
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	if owned > 0 {
		page.State = "owner"
	} else {
		var status string
		err := h.DB.QueryRow(`
			SELECT status, admin_note, business_name, phone
			FROM vendor_applications
			WHERE user_id = $1
			ORDER BY id DESC LIMIT 1
		`, userID).Scan(&status, &page.AdminNote, &page.BusinessName, &page.Phone)

		switch {
		case err == sql.ErrNoRows:
			page.State = "form"
		case err != nil:
			log.Printf("vendor apply: application lookup failed: %v", err)
			errors.Render(w, "Something went wrong", http.StatusInternalServerError)
			return
		case status == "pending":
			page.State = "pending"
		case status == "rejected":
			page.State = "rejected"
		default:
			page.State = "form"
		}
	}

	tmpl, err := template.New("become-vendor.html").
		Funcs(template.FuncMap{"naira": naira}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/become-vendor.html")
	if err != nil {
		log.Printf("vendor apply: template failed: %v", err)
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor apply: execute failed: %v", err)
	}
}

func (h *Handlers) VendorApplySubmit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.sessionUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}

	back := func(msg string) {
		http.Redirect(w, r, "/become-a-vendor?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	var owned int
	h.DB.QueryRow("SELECT count(*) FROM vendors WHERE owner_user_id = $1", userID).Scan(&owned)
	if owned > 0 {
		back("You already have a shop.")
		return
	}

	business := strings.TrimSpace(r.FormValue("business_name"))
	owner := strings.Join(strings.Fields(r.FormValue("owner_name")), " ")
	phone := strings.TrimSpace(r.FormValue("phone"))
	area := strings.TrimSpace(r.FormValue("area"))
	address := strings.TrimSpace(r.FormValue("address"))
	idType := r.FormValue("id_type")
	link := strings.TrimSpace(r.FormValue("online_link"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	if n := len([]rune(business)); n < 3 || n > 80 {
		back("The business name must be between 3 and 80 characters.")
		return
	}
	slug := slugify(business)
	if slug == "" {
		back("The business name needs some letters or numbers.")
		return
	}
	var taken bool
	if err := h.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM vendors WHERE slug = $1)", slug).Scan(&taken); err != nil {
		log.Printf("vendor apply: slug check failed: %v", err)
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if taken {
		back("A shop with a very similar name already exists. Please choose a different name.")
		return
	}

	if msg := validFullName(owner); msg != "" {
		back(msg + ".")
		return
	}

	normalized := normalizeNigerianPhone(phone)
	if normalized == "" {
		back("Enter a valid Nigerian phone number: 11 digits starting with 0 (for example 08012345678), or starting with +234.")
		return
	}

	if n := len([]rune(area)); n < 2 || n > 60 {
		back("Enter the area your shop is in (2 to 60 characters).")
		return
	}
	if n := len([]rune(address)); n < 8 || n > 150 {
		back("Enter your shop address (8 to 150 characters).")
		return
	}
	if !validIDType(idType) {
		back("Choose the kind of ID you will show us.")
		return
	}

	if link != "" {
		u, err := url.Parse(link)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(link) > 300 {
			back("The online link must start with https:// (or leave it empty).")
			return
		}
	}
	if len([]rune(notes)) > 500 {
		back("The notes must be 500 characters or fewer.")
		return
	}
	if r.FormValue("confirm") != "on" {
		back("Please confirm that your details are true and that you agree to be contacted.")
		return
	}

	_, err := h.DB.Exec(`
		INSERT INTO vendor_applications
		(user_id, business_name, owner_name, phone, area, address, id_type, online_link, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, userID, business, owner, normalized, area, address, idType, link, notes)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			back("You already have an application waiting for review.")
			return
		}
		log.Printf("vendor apply: insert failed: %v", err)
		errors.Render(w, "Could not save your application", http.StatusInternalServerError)
		return
	}

	back("Application received. We will review it and contact you.")
}
