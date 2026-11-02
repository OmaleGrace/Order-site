package handlers

import (
	"html/template"
	"log"
	"net/http"
	"net/url"

	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

// vendorAgreementVersion is saved with every acceptance. Change it when the agreement text changes,
// so you can always tell which wording a vendor agreed to.
const vendorAgreementVersion = "2026-11-01"

type VendorAgreementPage struct {
	Version           string
	SetupFeeKobo      int
	CommissionPercent int
	HoldHours         int
}

// VendorAgreement shows the vendor agreement. It is public, so anyone can read it before applying.
func (h *Handlers) VendorAgreement(w http.ResponseWriter, r *http.Request) {
	page := VendorAgreementPage{
		Version:           vendorAgreementVersion,
		SetupFeeKobo:      vendorSetupFeeKobo(),
		CommissionPercent: commissionPercent(),
		HoldHours:         payoutHoldHours(),
	}
	tmpl, err := render.Page("vendor-agreement.html", template.FuncMap{"naira": naira})
	if err != nil {
		log.Printf("vendor agreement: template failed: %v", err)
		errors.Render(w, "Could not display the agreement", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor agreement: execute failed: %v", err)
	}
}

// VendorAgreementAccept records that the signed-in applicant has read and accepted the agreement.
// It is for applications that were sent before the agreement existed.
func (h *Handlers) VendorAgreementAccept(w http.ResponseWriter, r *http.Request) {
	userID, err := requestUserID(h, r)
	if err != nil {
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
	if r.FormValue("agree") != "on" {
		back("Tick the box to accept the vendor agreement.")
		return
	}
	res, err := h.DB.Exec(`
		UPDATE vendor_applications
		SET agreement_version = $2, agreement_accepted_at = NOW(), agreement_ip = $3
		WHERE user_id = $1 AND status = 'pending' AND agreement_accepted_at IS NULL
	`, userID, vendorAgreementVersion, middleware.ClientIP(r))
	if err != nil {
		log.Printf("vendor agreement: accept failed: %v", err)
		errors.Render(w, "Could not save your acceptance", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		back("There is no application waiting for the agreement.")
		return
	}
	back("Thank you. You have accepted the vendor agreement.")
}
