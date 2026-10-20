package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"Order-site/brand"
	"Order-site/email"
	"Order-site/errors"
)

var vendorPayClient = &http.Client{Timeout: 20 * time.Second}

var (
	errVendorPaymentUnknown = stderrors.New("vendor payment not found")
	errVendorPaymentAmount  = stderrors.New("vendor payment amount mismatch")
)

type vendorBilling struct {
	Name      string
	Status    string
	SetupPaid bool
	PaidUntil string
	Current   bool
	TooFar    bool
	NoExpiry  bool
}

func (h *Handlers) loadVendorBilling(vendorID int) (vendorBilling, error) {
	var b vendorBilling
	err := h.DB.QueryRow(`
		SELECT name, status, setup_paid_at IS NOT NULL,
		       COALESCE(TO_CHAR(paid_until, 'DD Mon YYYY'), ''),
		       COALESCE(paid_until > NOW(), false),
		       COALESCE(paid_until > NOW() + INTERVAL '90 days', false),
		       COALESCE(paid_until > NOW() + INTERVAL '50 years', false)
		FROM vendors WHERE id = $1
	`, vendorID).Scan(&b.Name, &b.Status, &b.SetupPaid, &b.PaidUntil, &b.Current, &b.TooFar, &b.NoExpiry)
	return b, err
}

// billingKind says which payment, if any, this shop can make right now.
func billingKind(b vendorBilling) (kind string, reason string) {
	switch {
	case b.Status == "suspended":
		return "", "Your shop is suspended. Please contact the site admin."
	case !b.SetupPaid && b.Status == "pending":
		return "setup", ""
	case b.SetupPaid:
		return "", "Nothing to pay. There is no monthly fee."
	}
	return "", "Payments are not available for this shop right now."
}

type VendorPaymentRow struct {
	Kind       string
	AmountKobo int
	PaidAt     string
}

type VendorBillingPage struct {
	Name              string
	State             string // setup, active, suspended
	CanPay            bool
	PayKind           string
	PayAmountKobo     int
	Reason            string
	SetupFeeKobo      int
	CommissionPercent int
	Notice            string
	History           []VendorPaymentRow
	Earnings          VendorEarnings
	Payouts           []VendorPayoutRow
}

func (h *Handlers) VendorBilling(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	b, err := h.loadVendorBilling(vendorID)
	if err != nil {
		log.Printf("vendor billing: load failed: %v", err)
		errors.Render(w, "Could not load your billing details", http.StatusInternalServerError)
		return
	}

	page := VendorBillingPage{
		Name:              b.Name,
		SetupFeeKobo:      vendorSetupFeeKobo(),
		CommissionPercent: commissionPercent(),
		Notice:            r.URL.Query().Get("msg"),
	}

	switch {
	case b.Status == "suspended":
		page.State = "suspended"
	case !b.SetupPaid:
		page.State = "setup"
	default:
		page.State = "active"
	}

	if e, err := h.vendorEarnings(vendorID); err != nil {
		log.Printf("vendor billing: earnings failed: %v", err)
	} else {
		page.Earnings = e
	}
	page.Payouts = h.vendorPayouts(vendorID, 10)

	kind, reason := billingKind(b)
	page.PayKind = kind
	page.CanPay = kind != ""
	page.Reason = reason
	if kind == "setup" {
		page.PayAmountKobo = page.SetupFeeKobo
	}

	rows, err := h.DB.Query(`
		SELECT kind, amount_kobo, TO_CHAR(paid_at, 'DD Mon YYYY')
		FROM vendor_payments
		WHERE vendor_id = $1 AND status = 'paid'
		ORDER BY paid_at DESC LIMIT 10
	`, vendorID)
	if err != nil {
		log.Printf("vendor billing: history failed: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var p VendorPaymentRow
			if err := rows.Scan(&p.Kind, &p.AmountKobo, &p.PaidAt); err == nil {
				page.History = append(page.History, p)
			}
		}
	}

	tmpl, err := template.New("vendor-billing.html").
		Funcs(template.FuncMap{"naira": naira, "money": money}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/vendor-billing.html")
	if err != nil {
		log.Printf("vendor billing: template failed: %v", err)
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor billing: execute failed: %v", err)
	}
}

// VendorPay starts a Paystack payment for the shop's setup fee or a renewal.
func (h *Handlers) VendorPay(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	userID, ok := h.sessionUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	back := func(msg string) {
		http.Redirect(w, r, "/vendor/billing?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	b, err := h.loadVendorBilling(vendorID)
	if err != nil {
		log.Printf("vendor pay: load failed: %v", err)
		errors.Render(w, "Could not start the payment", http.StatusInternalServerError)
		return
	}

	kind, reason := billingKind(b)
	if kind == "" {
		back(reason)
		return
	}

	amount := vendorSetupFeeKobo()

	base := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/")
	if base == "" {
		log.Printf("vendor pay: APP_BASE_URL is not set")
		errors.Render(w, "Payments are not set up on this site yet.", http.StatusInternalServerError)
		return
	}

	var userEmail string
	if err := h.DB.QueryRow("SELECT email FROM users WHERE id = $1", userID).Scan(&userEmail); err != nil {
		log.Printf("vendor pay: email lookup failed: %v", err)
		errors.Render(w, "Could not start the payment", http.StatusInternalServerError)
		return
	}

	reference := fmt.Sprintf("VENDOR-%d-%d", vendorID, time.Now().UnixNano())

	if _, err := h.DB.Exec(`
		INSERT INTO vendor_payments (vendor_id, kind, amount_kobo, reference)
		VALUES ($1, $2, $3, $4)
	`, vendorID, kind, amount, reference); err != nil {
		log.Printf("vendor pay: insert failed: %v", err)
		errors.Render(w, "Could not start the payment", http.StatusInternalServerError)
		return
	}

	body, _ := json.Marshal(map[string]interface{}{
		"email":        userEmail,
		"amount":       amount,
		"reference":    reference,
		"callback_url": base + "/vendor/payment/callback",
	})

	req, err := http.NewRequest(http.MethodPost, "https://api.paystack.co/transaction/initialize", bytes.NewBuffer(body))
	if err != nil {
		errors.Render(w, "Could not start the payment", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("PAYSTACK_SECRET_KEY"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := vendorPayClient.Do(req)
	if err != nil {
		log.Printf("vendor pay: paystack request failed: %v", err)
		h.DB.Exec("DELETE FROM vendor_payments WHERE reference = $1", reference)
		errors.Render(w, "Could not connect to the payment service", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var out struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			AuthorizationURL string `json:"authorization_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || !out.Status || out.Data.AuthorizationURL == "" {
		log.Printf("vendor pay: paystack refused: %v %s", err, out.Message)
		h.DB.Exec("DELETE FROM vendor_payments WHERE reference = $1", reference)
		errors.Render(w, "Could not start the payment", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, out.Data.AuthorizationURL, http.StatusSeeOther)
}

// verifyPaystackPayment asks Paystack whether a payment really succeeded.
func verifyPaystackPayment(reference string) (amountKobo int, success bool, err error) {
	req, err := http.NewRequest(http.MethodGet,
		"https://api.paystack.co/transaction/verify/"+url.PathEscape(reference), nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("PAYSTACK_SECRET_KEY"))

	resp, err := vendorPayClient.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()

	var out struct {
		Status bool `json:"status"`
		Data   struct {
			Status string `json:"status"`
			Amount int    `json:"amount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, false, err
	}
	return out.Data.Amount, out.Status && strings.ToLower(out.Data.Status) == "success", nil
}

// activateVendorPayment records a confirmed payment and extends the shop's paid period.
// It is safe to call twice for the same payment: the second call changes nothing.
func (h *Handlers) activateVendorPayment(reference string, amountKobo int) (bool, error) {
	tx, err := h.DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var paymentID, vendorID, expected int
	var kind, status string
	err = tx.QueryRow(`
		SELECT id, vendor_id, kind, amount_kobo, status
		FROM vendor_payments
		WHERE reference = $1
		FOR UPDATE
	`, reference).Scan(&paymentID, &vendorID, &kind, &expected, &status)
	if err == sql.ErrNoRows {
		return false, errVendorPaymentUnknown
	}
	if err != nil {
		return false, err
	}

	if status == "paid" {
		return false, nil
	}
	if amountKobo != expected {
		return false, errVendorPaymentAmount
	}

	if _, err := tx.Exec(
		"UPDATE vendor_payments SET status = 'paid', paid_at = NOW() WHERE id = $1", paymentID,
	); err != nil {
		return false, err
	}

	// There is no monthly fee any more. Paying the setup fee opens the shop for good.
	// A shop you suspended stays suspended: only a waiting shop is opened.
	_, err = tx.Exec(`
		UPDATE vendors
		SET status = CASE WHEN status = 'pending' THEN 'active' ELSE status END,
		    setup_paid_at = COALESCE(setup_paid_at, NOW()),
		    paid_until = NOW() + INTERVAL '100 years'
		WHERE id = $1
	`, vendorID)
	if err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	var ownerEmail, shopName string
	h.DB.QueryRow(`
		SELECT u.email, v.name
		FROM vendors v
		JOIN users u ON u.id = v.owner_user_id
		WHERE v.id = $1
	`, vendorID).Scan(&ownerEmail, &shopName)

	if ownerEmail != "" {
		shop := template.HTMLEscapeString(shopName)
		go email.Send(ownerEmail, "Your shop is open on "+brand.Name(),
			fmt.Sprintf("<h1>Payment received</h1><p>Your shop <strong>%s</strong> is now open to customers. There is no monthly fee.</p>", shop))
	}

	return true, nil
}

// VendorPaymentCallback is where Paystack sends the vendor's browser after paying.
func (h *Handlers) VendorPaymentCallback(w http.ResponseWriter, r *http.Request) {
	reference := r.URL.Query().Get("reference")
	if !strings.HasPrefix(reference, "VENDOR-") {
		errors.Render(w, "Payment reference missing", http.StatusBadRequest)
		return
	}

	amount, success, err := verifyPaystackPayment(reference)
	if err != nil {
		log.Printf("vendor callback: verify failed: %v", err)
		errors.Render(w, "Could not verify the payment", http.StatusInternalServerError)
		return
	}
	if !success {
		http.Redirect(w, r, "/vendor/billing?msg="+url.QueryEscape("The payment was not completed. You have not been charged."), http.StatusSeeOther)
		return
	}

	if _, err := h.activateVendorPayment(reference, amount); err != nil {
		switch {
		case stderrors.Is(err, errVendorPaymentUnknown):
			errors.Render(w, "We could not find that payment.", http.StatusNotFound)
		case stderrors.Is(err, errVendorPaymentAmount):
			errors.Render(w, "The payment amount does not match.", http.StatusBadRequest)
		default:
			log.Printf("vendor callback: activate failed: %v", err)
			errors.Render(w, "Could not record the payment", http.StatusInternalServerError)
		}
		return
	}

	http.Redirect(w, r, "/vendor/billing?msg="+url.QueryEscape("Payment received. Thank you!"), http.StatusSeeOther)
}

// vendorWebhook handles a confirmed vendor payment announced by Paystack.
func (h *Handlers) vendorWebhook(w http.ResponseWriter, reference string, amountKobo int) {
	_, err := h.activateVendorPayment(reference, amountKobo)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case stderrors.Is(err, errVendorPaymentUnknown):
		errors.Render(w, "Payment not found", http.StatusNotFound)
	case stderrors.Is(err, errVendorPaymentAmount):
		errors.Render(w, "Payment amount mismatch", http.StatusBadRequest)
	default:
		log.Printf("webhook vendor payment: activate failed: %v", err)
		errors.Render(w, "Could not record the payment", http.StatusInternalServerError)
	}
}
