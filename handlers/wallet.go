package handlers

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"Order-site/brand"
	"Order-site/cart"
	"Order-site/email"
	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

// The wallet is a ledger: every credit or debit is a row in wallet_transactions and the
// balance is their sum. Rows are never edited, and each money event carries a unique
// reference, so the same payment can never be credited (or an order paid) twice.

var (
	errWalletInsufficient   = stderrors.New("wallet balance too low")
	errWalletPaymentUnknown = stderrors.New("wallet payment not found")
	errWalletPaymentAmount  = stderrors.New("wallet payment amount mismatch")
)

var giftRedeemLimiter = middleware.NewLimiter(10, time.Hour)

const (
	passDays        = 30
	minTopupKobo    = 50000    // N500
	maxTopupKobo    = 20000000 // N200,000
	minGiftCardKobo = 100000   // N1,000
	maxGiftCardKobo = 5000000  // N50,000
)

// passPriceKobo is the price of the 30-day LaikiHub Pass. Default N1,000.
func passPriceKobo() int { return envKobo("PASS_PRICE_KOBO", 100000) }

type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func walletBalance(q rowQuerier, userID int) int {
	var bal int
	_ = q.QueryRow(`SELECT COALESCE(SUM(amount_kobo), 0) FROM wallet_transactions WHERE user_id = $1`, userID).Scan(&bal)
	return bal
}

func (h *Handlers) hasPass(userID int) bool {
	var active bool
	_ = h.DB.QueryRow(`SELECT COALESCE(pass_until > NOW(), FALSE) FROM users WHERE id = $1`, userID).Scan(&active)
	return active
}

func newGiftCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)[:12]
	return s[0:4] + "-" + s[4:8] + "-" + s[8:12], nil
}

func normalizeGiftCode(raw string) string {
	s := strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(raw)))
	if len(s) != 12 {
		return ""
	}
	return s[0:4] + "-" + s[4:8] + "-" + s[8:12]
}

// ---------------------------------------------------------------------------
// Wallet page
// ---------------------------------------------------------------------------

type WalletRow struct {
	Label  string
	Note   string
	When   string
	Amount int // naira, always positive
	Credit bool
}

type WalletPage struct {
	Balance    int // naira
	PassActive bool
	PassUntil  string
	PassPrice  int // naira
	GiftMin    int
	GiftMax    int
	TopMin     int
	TopMax     int
	Notice     string
	History    []WalletRow
	Gifts      []GiftRow
}

type GiftRow struct {
	Code   string
	Amount int
	Used   bool
	When   string
}

var walletKindLabels = map[string]string{
	"topup": "Top up", "gift_redeemed": "Gift card", "order_payment": "Order payment", "refund": "Refund", "pass_payment": "Pass",
}

func (h *Handlers) Wallet(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		errors.Render(w, "Please log in to use your wallet.", http.StatusUnauthorized)
		return
	}
	page := WalletPage{
		Balance:   walletBalance(h.DB, userID) / 100,
		PassPrice: passPriceKobo() / 100,
		GiftMin:   minGiftCardKobo / 100, GiftMax: maxGiftCardKobo / 100,
		TopMin: minTopupKobo / 100, TopMax: maxTopupKobo / 100,
		Notice: r.URL.Query().Get("msg"),
	}
	var until sql.NullString
	_ = h.DB.QueryRow(`SELECT COALESCE(pass_until > NOW(), FALSE), TO_CHAR(pass_until, 'DD Mon YYYY') FROM users WHERE id = $1`, userID).Scan(&page.PassActive, &until)
	if page.PassActive && until.Valid {
		page.PassUntil = until.String
	}

	rows, err := h.DB.Query(`
		SELECT kind, note, amount_kobo, TO_CHAR(created_at, 'DD Mon YYYY HH24:MI')
		FROM wallet_transactions WHERE user_id = $1 ORDER BY id DESC LIMIT 25`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var kind, note, when string
			var amt int
			if rows.Scan(&kind, &note, &amt, &when) == nil {
				label := walletKindLabels[kind]
				if label == "" {
					label = "Wallet"
				}
				if amt < 0 {
					amt = -amt
				}
				page.History = append(page.History, WalletRow{Label: label, Note: note, When: when, Amount: amt / 100, Credit: kind != "order_payment" && kind != "pass_payment"})
			}
		}
	}
	grows, err := h.DB.Query(`
		SELECT code, amount_kobo, redeemed_at IS NOT NULL, TO_CHAR(created_at, 'DD Mon YYYY')
		FROM gift_cards WHERE buyer_user_id = $1 ORDER BY id DESC LIMIT 10`, userID)
	if err == nil {
		defer grows.Close()
		for grows.Next() {
			var g GiftRow
			var amt int
			if grows.Scan(&g.Code, &amt, &g.Used, &g.When) == nil {
				g.Amount = amt / 100
				page.Gifts = append(page.Gifts, g)
			}
		}
	}

	tmpl, err := render.Page("wallet.html", template.FuncMap{})
	if err != nil {
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, page)
}

// ---------------------------------------------------------------------------
// Paying Paystack for a top-up, a gift card or the pass
// ---------------------------------------------------------------------------

func paystackInitialize(emailAddr string, amountKobo int, reference, callback string) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"email": emailAddr, "amount": amountKobo, "reference": reference, "callback_url": callback,
	})
	req, err := http.NewRequest(http.MethodPost, "https://api.paystack.co/transaction/initialize", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("PAYSTACK_SECRET_KEY"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := vendorPayClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			AuthorizationURL string `json:"authorization_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if !out.Status || out.Data.AuthorizationURL == "" {
		return "", fmt.Errorf("paystack refused: %s", out.Message)
	}
	return out.Data.AuthorizationURL, nil
}

func (h *Handlers) WalletPay(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		http.Redirect(w, r, "/login?next=/wallet", http.StatusSeeOther)
		return
	}
	back := func(msg string) {
		http.Redirect(w, r, "/wallet?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		back("Something went wrong. Please try again.")
		return
	}

	kind := r.FormValue("kind")
	var amount int
	var recipient, message string
	switch kind {
	case "topup":
		n, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("amount")))
		amount = n * 100
		if amount < minTopupKobo || amount > maxTopupKobo {
			back(fmt.Sprintf("Enter a top-up between ₦%d and ₦%d.", minTopupKobo/100, maxTopupKobo/100))
			return
		}
	case "giftcard":
		n, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("amount")))
		amount = n * 100
		if amount < minGiftCardKobo || amount > maxGiftCardKobo {
			back(fmt.Sprintf("A gift card must be between ₦%d and ₦%d.", minGiftCardKobo/100, maxGiftCardKobo/100))
			return
		}
		recipient = strings.ToLower(strings.TrimSpace(r.FormValue("recipient_email")))
		if recipient != "" && (len(recipient) > 254 || validateEmail(recipient) != nil) {
			back("That recipient email doesn't look right. Leave it empty to get the code yourself.")
			return
		}
		message = strings.TrimSpace(r.FormValue("message"))
		if len([]rune(message)) > 200 {
			message = string([]rune(message)[:200])
		}
	case "pass":
		amount = passPriceKobo()
	default:
		back("Choose what you want to buy.")
		return
	}

	base := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/")
	if base == "" {
		log.Printf("wallet pay: APP_BASE_URL is not set")
		errors.Render(w, "Payments are not set up on this site yet.", http.StatusInternalServerError)
		return
	}
	var userEmail string
	if err := h.DB.QueryRow(`SELECT email FROM users WHERE id = $1 AND NOT is_guest`, userID).Scan(&userEmail); err != nil {
		errors.Render(w, "Could not start the payment", http.StatusInternalServerError)
		return
	}

	reference := fmt.Sprintf("WALLET-%d-%d", userID, time.Now().UnixNano())
	if _, err := h.DB.Exec(`
		INSERT INTO wallet_payments (reference, user_id, kind, amount_kobo, recipient_email, message)
		VALUES ($1, $2, $3, $4, $5, $6)`, reference, userID, kind, amount, recipient, message); err != nil {
		log.Printf("wallet pay: insert failed: %v", err)
		errors.Render(w, "Could not start the payment", http.StatusInternalServerError)
		return
	}
	authURL, err := paystackInitialize(userEmail, amount, reference, base+"/wallet/callback")
	if err != nil {
		log.Printf("wallet pay: paystack failed: %v", err)
		h.DB.Exec(`DELETE FROM wallet_payments WHERE reference = $1`, reference)
		errors.Render(w, "Could not connect to the payment service", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

// fulfilWalletPayment applies a confirmed payment exactly once: it credits the wallet, makes
// the gift card or extends the pass. Calling it again for the same payment changes nothing.
func (h *Handlers) fulfilWalletPayment(reference string, amountKobo int) (bool, error) {
	tx, err := h.DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var userID, expected int
	var kind, status, recipient, message string
	err = tx.QueryRow(`
		SELECT user_id, kind, amount_kobo, status, recipient_email, message
		FROM wallet_payments WHERE reference = $1 FOR UPDATE`, reference).Scan(&userID, &kind, &expected, &status, &recipient, &message)
	if err == sql.ErrNoRows {
		return false, errWalletPaymentUnknown
	}
	if err != nil {
		return false, err
	}
	if status == "paid" {
		return false, nil
	}
	if amountKobo != expected {
		return false, errWalletPaymentAmount
	}
	if _, err := tx.Exec(`UPDATE wallet_payments SET status = 'paid', paid_at = NOW() WHERE reference = $1`, reference); err != nil {
		return false, err
	}

	// Serialise this user's money movements.
	if _, err := tx.Exec(`SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
		return false, err
	}

	var giftCode string
	switch kind {
	case "topup":
		if _, err := tx.Exec(`INSERT INTO wallet_transactions (user_id, amount_kobo, kind, reference, note) VALUES ($1, $2, 'topup', $3, 'Paystack top-up')`,
			userID, expected, reference); err != nil {
			return false, err
		}
	case "giftcard":
		for tries := 0; tries < 5; tries++ {
			code, cerr := newGiftCode()
			if cerr != nil {
				return false, cerr
			}
			res, ierr := tx.Exec(`
				INSERT INTO gift_cards (code, amount_kobo, buyer_user_id, recipient_email, message, payment_reference)
				VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (code) DO NOTHING`, code, expected, userID, recipient, message, reference)
			if ierr != nil {
				return false, ierr
			}
			if n, _ := res.RowsAffected(); n == 1 {
				giftCode = code
				break
			}
		}
		if giftCode == "" {
			return false, fmt.Errorf("could not make a gift code")
		}
	case "pass":
		if _, err := tx.Exec(`UPDATE users SET pass_until = GREATEST(COALESCE(pass_until, NOW()), NOW()) + make_interval(days => $2) WHERE id = $1`, userID, passDays); err != nil {
			return false, err
		}
	default:
		return false, fmt.Errorf("unknown wallet payment kind %q", kind)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}

	var buyerEmail, buyerName string
	_ = h.DB.QueryRow(`SELECT email, name FROM users WHERE id = $1`, userID).Scan(&buyerEmail, &buyerName)
	switch kind {
	case "giftcard":
		cardHTML := fmt.Sprintf(`<h1>A %s gift card</h1><p style="font-size:22px;letter-spacing:2px"><strong>%s</strong></p><p>Worth ₦%d. Log in, open <strong>Wallet</strong> and enter this code to add it to your balance.</p>`,
			template.HTMLEscapeString(brand.Name()), template.HTMLEscapeString(giftCode), expected/100)
		if recipient != "" {
			note := ""
			if message != "" {
				note = "<p><em>“" + template.HTMLEscapeString(message) + "”</em></p>"
			}
			go email.Send(recipient, buyerName+" sent you a gift card", cardHTML+note)
		}
		if buyerEmail != "" {
			extra := "<p>We've emailed it to your recipient."
			if recipient == "" {
				extra = "<p>Share this code with whoever you want to gift it to."
			}
			go email.Send(buyerEmail, "Your gift card", cardHTML+extra+"</p>")
		}
	case "topup":
		if buyerEmail != "" {
			go email.Send(buyerEmail, "Wallet top-up received", fmt.Sprintf("<h1>Top-up received</h1><p>₦%d was added to your %s wallet.</p>", expected/100, template.HTMLEscapeString(brand.Name())))
		}
	case "pass":
		if buyerEmail != "" {
			go email.Send(buyerEmail, "Your pass is active", fmt.Sprintf("<h1>Pass active</h1><p>Your %s Pass is active for %d days: no service fee and double reward points on your orders.</p>", template.HTMLEscapeString(brand.Name()), passDays))
		}
	}
	return true, nil
}

func (h *Handlers) WalletCallback(w http.ResponseWriter, r *http.Request) {
	reference := r.URL.Query().Get("reference")
	if !strings.HasPrefix(reference, "WALLET-") {
		errors.Render(w, "Payment reference missing", http.StatusBadRequest)
		return
	}
	amount, success, err := verifyPaystackPayment(reference)
	if err != nil {
		log.Printf("wallet callback: verify failed: %v", err)
		errors.Render(w, "Could not verify the payment", http.StatusInternalServerError)
		return
	}
	if !success {
		http.Redirect(w, r, "/wallet?msg="+url.QueryEscape("The payment was not completed. You have not been charged."), http.StatusSeeOther)
		return
	}
	if _, err := h.fulfilWalletPayment(reference, amount); err != nil {
		switch {
		case stderrors.Is(err, errWalletPaymentUnknown):
			errors.Render(w, "We could not find that payment.", http.StatusNotFound)
		case stderrors.Is(err, errWalletPaymentAmount):
			errors.Render(w, "The payment amount does not match.", http.StatusBadRequest)
		default:
			log.Printf("wallet callback: fulfil failed: %v", err)
			errors.Render(w, "Could not record the payment", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/wallet?msg="+url.QueryEscape("Payment received. Thank you!"), http.StatusSeeOther)
}

func (h *Handlers) walletWebhook(w http.ResponseWriter, reference string, amountKobo int) {
	_, err := h.fulfilWalletPayment(reference, amountKobo)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case stderrors.Is(err, errWalletPaymentUnknown):
		errors.Render(w, "Payment not found", http.StatusNotFound)
	case stderrors.Is(err, errWalletPaymentAmount):
		errors.Render(w, "Payment amount mismatch", http.StatusBadRequest)
	default:
		log.Printf("webhook wallet payment: fulfil failed: %v", err)
		errors.Render(w, "Could not record the payment", http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// Redeeming a gift card
// ---------------------------------------------------------------------------

func (h *Handlers) WalletRedeem(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		http.Redirect(w, r, "/login?next=/wallet", http.StatusSeeOther)
		return
	}
	back := func(msg string) {
		http.Redirect(w, r, "/wallet?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	if !giftRedeemLimiter.Hit(strconv.Itoa(userID)) {
		back("Too many attempts. Please try again later.")
		return
	}
	if err := r.ParseForm(); err != nil {
		back("Something went wrong. Please try again.")
		return
	}
	code := normalizeGiftCode(r.FormValue("code"))
	if code == "" {
		back("That doesn't look like a gift card code. It has 12 letters and numbers, like ABCD-EFGH-JKLM.")
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		back("Could not redeem the code. Please try again.")
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
		back("Could not redeem the code. Please try again.")
		return
	}
	var cardID, amount int
	err = tx.QueryRow(`
		UPDATE gift_cards SET redeemed_by = $1, redeemed_at = NOW()
		WHERE code = $2 AND redeemed_at IS NULL RETURNING id, amount_kobo`, userID, code).Scan(&cardID, &amount)
	if err == sql.ErrNoRows {
		back("That code isn't valid, or it has already been used.")
		return
	}
	if err != nil {
		log.Printf("wallet redeem: update failed: %v", err)
		back("Could not redeem the code. Please try again.")
		return
	}
	if _, err := tx.Exec(`INSERT INTO wallet_transactions (user_id, amount_kobo, kind, reference, note) VALUES ($1, $2, 'gift_redeemed', $3, 'Gift card redeemed')`,
		userID, amount, fmt.Sprintf("GIFT-%d", cardID)); err != nil {
		log.Printf("wallet redeem: credit failed: %v", err)
		back("Could not redeem the code. Please try again.")
		return
	}
	if err := tx.Commit(); err != nil {
		back("Could not redeem the code. Please try again.")
		return
	}
	back(fmt.Sprintf("₦%d was added to your wallet.", amount/100))
}

// ---------------------------------------------------------------------------
// Paying an order from the wallet, and refunds
// ---------------------------------------------------------------------------

// payOrderWithWallet pays a pending order in full from the customer's wallet. The amount
// comes from the order itself, never from the request. It is all-or-nothing: the debit and
// marking the order paid happen in one transaction.
func (h *Handlers) payOrderWithWallet(orderID, userID int) error {
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
		return err
	}
	var amount, loyaltyPoints int
	if err := tx.QueryRow(`
		SELECT total_kobo + service_fee_kobo, loyalty_points_redeemed
		FROM orders WHERE id = $1 AND user_id = $2 AND status = 'pending' FOR UPDATE`, orderID, userID).Scan(&amount, &loyaltyPoints); err != nil {
		return err
	}
	if walletBalance(tx, userID) < amount {
		return errWalletInsufficient
	}
	if amount > 0 {
		if _, err := tx.Exec(`INSERT INTO wallet_transactions (user_id, amount_kobo, kind, reference, order_id, note)
			VALUES ($1, $2, 'order_payment', $3, $4, 'Paid for an order')`,
			userID, -amount, fmt.Sprintf("ORDER-%d", orderID), orderID); err != nil {
			return err
		}
	}
	res, err := tx.Exec(`
		UPDATE orders
		SET status = CASE WHEN scheduled_for IS NOT NULL AND scheduled_for > NOW() THEN 'scheduled' ELSE 'paid' END,
		    payment_reference = $2
		WHERE id = $1 AND status = 'pending'`, orderID, fmt.Sprintf("WALLET-ORDER-%d", orderID))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("order %d is no longer pending", orderID)
	}
	if loyaltyPoints > 0 {
		if err := redeemLoyaltyForOrder(tx, userID, orderID, loyaltyPoints); err != nil {
			return err
		}
	}
	if err := settleCoupon(tx, orderID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Everything a card payment does once it is confirmed.
	if err := cart.ClearCart(h.DB, userID); err != nil {
		log.Printf("wallet order %d: clear cart failed: %v", orderID, err)
	}
	go func(id int) {
		if err := sendOrderPlacedEmail(h.DB, id); err != nil {
			log.Printf("wallet order %d: confirmation email failed: %v", id, err)
		}
	}(orderID)
	var current string
	_ = h.DB.QueryRow(`SELECT status FROM orders WHERE id = $1`, orderID).Scan(&current)
	title, msg := "Order confirmed", "Your payment was confirmed and your order is now being processed."
	if current == "scheduled" {
		title, msg = "Scheduled order confirmed", "Your payment was confirmed. Your order will become active at the scheduled time."
		if err := notifyVendorScheduledOrder(h.DB, orderID); err != nil {
			log.Printf("wallet order %d: vendor scheduled notification failed: %v", orderID, err)
		}
	} else {
		go notifyVendorNewOrder(h.DB, orderID)
	}
	if err := createNotification(h.DB, userID, "order", title, msg, "/orders"); err != nil {
		log.Printf("wallet order %d: notification failed: %v", orderID, err)
	}
	return nil
}

// refundWalletOrder puts the money back in the wallet when an order that was paid from the
// wallet is cancelled. The unique reference means it can only ever happen once per order.
func (h *Handlers) refundWalletOrder(orderID int) {
	var userID, amount int
	err := h.DB.QueryRow(`
		SELECT user_id, total_kobo + service_fee_kobo FROM orders
		WHERE id = $1 AND payment_reference = 'WALLET-ORDER-' || id::text`, orderID).Scan(&userID, &amount)
	if err != nil || amount <= 0 {
		return
	}
	tx, err := h.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
		return
	}
	if _, err := tx.Exec(`
		INSERT INTO wallet_transactions (user_id, amount_kobo, kind, reference, order_id, note)
		VALUES ($1, $2, 'refund', $3, $4, 'Refund for a cancelled order')
		ON CONFLICT (reference) DO NOTHING`, userID, amount, fmt.Sprintf("REFUND-%d", orderID), orderID); err != nil {
		log.Printf("wallet refund order %d failed: %v", orderID, err)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("wallet refund order %d commit failed: %v", orderID, err)
	}
}
