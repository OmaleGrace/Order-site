package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"Order-site/errors"
)

// payoutHoldHours is how long money from a completed order stays on hold before it can be sent to
// the vendor. The hold gives customers time to report a problem. Set PAYOUT_HOLD_HOURS to change it
// (0 to 720); the default is 24.
func payoutHoldHours() int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("PAYOUT_HOLD_HOURS"))); err == nil && v >= 0 && v <= 720 {
		return v
	}
	return 24
}

// payoutPosition splits what a vendor has earned (sales minus our commission) into the part that
// can be sent now, the part on hold after completion, and the part for orders still in progress.
type payoutPosition struct {
	AvailableKobo  int // completed orders past the hold, minus what was already sent
	HeldKobo       int // completed orders still inside the hold
	InProgressKobo int // paid orders that are not completed yet
	PaidKobo       int // sent or being sent (failed transfers do not count)
}

type rowScanner interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// settlePosition applies payouts that were already sent: they come out of the available money
// first and, if more than that was sent, out of the held and in-progress money.
func settlePosition(availGross, held, inProgress, paid int) payoutPosition {
	p := payoutPosition{PaidKobo: paid}
	remaining := paid
	take := func(v int) int {
		t := v
		if t > remaining {
			t = remaining
		}
		remaining -= t
		return v - t
	}
	p.AvailableKobo = take(availGross)
	p.HeldKobo = take(held)
	p.InProgressKobo = take(inProgress)
	return p
}

func loadPayoutPosition(ctx context.Context, q rowScanner, vendorID int) (payoutPosition, error) {
	var availGross, held, inProgress, paid int
	err := q.QueryRowContext(ctx, `
		SELECT
		  COALESCE(SUM(total_kobo - commission_kobo) FILTER (WHERE status = 'completed' AND completed_at IS NOT NULL
		           AND completed_at <= NOW() - make_interval(hours => $2::int)), 0),
		  COALESCE(SUM(total_kobo - commission_kobo) FILTER (WHERE status = 'completed'
		           AND (completed_at IS NULL OR completed_at > NOW() - make_interval(hours => $2::int))), 0),
		  COALESCE(SUM(total_kobo - commission_kobo) FILTER (WHERE status NOT IN ('pending', 'cancelled', 'completed')), 0),
		  (SELECT COALESCE(SUM(amount_kobo), 0) FROM vendor_payouts WHERE vendor_id = $1 AND status <> 'failed')
		FROM orders WHERE vendor_id = $1
	`, vendorID, payoutHoldHours()).Scan(&availGross, &held, &inProgress, &paid)
	if err != nil {
		return payoutPosition{}, err
	}
	return settlePosition(availGross, held, inProgress, paid), nil
}

var paystackTransferClient = &http.Client{Timeout: 25 * time.Second}

// paystackJSON sends a JSON request to Paystack and decodes the answer into out.
func paystackJSON(ctx context.Context, method, path string, payload any, out any) (int, error) {
	secret := strings.TrimSpace(os.Getenv("PAYSTACK_SECRET_KEY"))
	if secret == "" {
		return 0, fmt.Errorf("PAYSTACK_SECRET_KEY is not set")
	}
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.paystack.co"+path, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := paystackTransferClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return resp.StatusCode, fmt.Errorf("unreadable Paystack response (HTTP %d)", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

type paystackEnvelope struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		RecipientCode string `json:"recipient_code"`
		TransferCode  string `json:"transfer_code"`
		Status        string `json:"status"`
		Reference     string `json:"reference"`
	} `json:"data"`
}

// AdminTransferPayout sends a vendor's available money to their verified bank account through
// Paystack Transfers. The payout is recorded first, as "processing"; Paystack's webhook later marks
// it paid or failed. Needs Transfers switched on in the Paystack dashboard, with the setting that
// asks for an OTP on every transfer turned off.
func (h *Handlers) AdminTransferPayout(w http.ResponseWriter, r *http.Request) {
	adminID, _ := h.sessionUserID(r)
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}
	back := func(msg string) {
		http.Redirect(w, r, "/admin/earnings?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	vendorID, err := strconv.Atoi(r.FormValue("vendor_id"))
	if err != nil {
		errors.Render(w, "Invalid vendor", http.StatusBadRequest)
		return
	}
	naira, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("amount")), 64)
	if err != nil || naira < 100 || naira > 100000000 {
		back("Enter an amount of at least ₦100.")
		return
	}
	amount := int(math.Round(naira * 100))

	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		errors.Render(w, "Could not start the transfer", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var name, bankCode, accountNumber, accountName, recipient, recipientAccount string
	var verified bool
	err = tx.QueryRowContext(ctx, `
		SELECT name, COALESCE(payout_bank_code,''), COALESCE(payout_account_number,''), COALESCE(payout_account_name,''),
		       COALESCE(payout_account_verified, FALSE), COALESCE(paystack_recipient_code,''), COALESCE(paystack_recipient_account,'')
		FROM vendors WHERE id = $1 FOR UPDATE
	`, vendorID).Scan(&name, &bankCode, &accountNumber, &accountName, &verified, &recipient, &recipientAccount)
	if err == sql.ErrNoRows {
		back("That vendor was not found.")
		return
	}
	if err != nil {
		log.Printf("payout transfer: vendor lookup failed: %v", err)
		errors.Render(w, "Could not start the transfer", http.StatusInternalServerError)
		return
	}
	if !verified || bankCode == "" || accountNumber == "" {
		back("The vendor must verify their bank account first.")
		return
	}
	pos, err := loadPayoutPosition(ctx, tx, vendorID)
	if err != nil {
		log.Printf("payout transfer: position failed: %v", err)
		errors.Render(w, "Could not start the transfer", http.StatusInternalServerError)
		return
	}
	if amount > pos.AvailableKobo {
		back("Only ₦" + money(pos.AvailableKobo) + " is available now. The rest is on hold for " + strconv.Itoa(payoutHoldHours()) + " hours after each order is completed, or the orders are not completed yet.")
		return
	}

	// Paystack needs a recipient first. Reuse it unless the vendor changed their account.
	if recipient == "" || recipientAccount != accountNumber {
		var rec paystackEnvelope
		status, rerr := paystackJSON(ctx, http.MethodPost, "/transferrecipient", map[string]string{
			"type": "nuban", "name": accountName, "account_number": accountNumber, "bank_code": bankCode, "currency": "NGN",
		}, &rec)
		if rerr != nil || !rec.Status || rec.Data.RecipientCode == "" {
			log.Printf("payout transfer: recipient failed (HTTP %d): %v %s", status, rerr, rec.Message)
			msg := rec.Message
			if msg == "" {
				msg = "Paystack could not be reached."
			}
			back("Could not set up the bank recipient: " + msg)
			return
		}
		recipient = rec.Data.RecipientCode
		if _, uerr := tx.ExecContext(ctx, `UPDATE vendors SET paystack_recipient_code = $1, paystack_recipient_account = $2 WHERE id = $3`, recipient, accountNumber, vendorID); uerr != nil {
			log.Printf("payout transfer: saving recipient failed: %v", uerr)
			errors.Render(w, "Could not start the transfer", http.StatusInternalServerError)
			return
		}
	}

	reference := fmt.Sprintf("PAYOUT-%d-%d", vendorID, time.Now().UnixNano())
	var payoutID int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO vendor_payouts (vendor_id, amount_kobo, note, recorded_by, status, method, transfer_reference)
		VALUES ($1, $2, $3, $4, 'processing', 'paystack', $5) RETURNING id
	`, vendorID, amount, "Paystack transfer", adminID, reference).Scan(&payoutID)
	if err != nil {
		log.Printf("payout transfer: insert failed: %v", err)
		errors.Render(w, "Could not start the transfer", http.StatusInternalServerError)
		return
	}
	// Commit before calling Paystack: the payout now counts against the balance, so a second click
	// cannot send the same money twice, even if the request below is slow or the page is reloaded.
	if err := tx.Commit(); err != nil {
		log.Printf("payout transfer: commit failed: %v", err)
		errors.Render(w, "Could not start the transfer", http.StatusInternalServerError)
		return
	}

	var tr paystackEnvelope
	status, terr := paystackJSON(ctx, http.MethodPost, "/transfer", map[string]any{
		"source": "balance", "amount": amount, "recipient": recipient, "reference": reference,
		"reason": "Sales payout for " + name,
	}, &tr)
	if terr != nil && status == 0 {
		// Could not tell whether Paystack received it. Leave it as processing: the webhook will settle it,
		// and the admin can check the Paystack dashboard.
		log.Printf("payout transfer: request error, leaving as processing: %v", terr)
		back("The transfer request was sent but Paystack did not answer. It is marked as processing; check your Paystack dashboard before trying again.")
		return
	}
	if terr != nil || !tr.Status {
		reason := tr.Message
		if reason == "" && terr != nil {
			reason = terr.Error()
		}
		_, _ = h.DB.Exec(`UPDATE vendor_payouts SET status = 'failed', failure_reason = $1 WHERE id = $2`, reason, payoutID)
		log.Printf("payout transfer: Paystack refused: %s", reason)
		back("Paystack did not send the money: " + reason)
		return
	}
	_, _ = h.DB.Exec(`UPDATE vendor_payouts SET transfer_code = $1 WHERE id = $2`, tr.Data.TransferCode, payoutID)
	log.Printf("payout transfer: admin=%d vendor=%d amount_kobo=%d ref=%s status=%s", adminID, vendorID, amount, reference, tr.Data.Status)
	if tr.Data.Status == "otp" {
		back("Paystack is waiting for an OTP. In Paystack Settings → Transfers, turn off the setting that asks for an OTP, then resend. This payout stays marked as processing.")
		return
	}
	back("Transfer started. It shows as paid once Paystack confirms it.")
}

// transferWebhook settles a payout from Paystack's transfer.success / transfer.failed / transfer.reversed events.
func (h *Handlers) transferWebhook(w http.ResponseWriter, event, reference, reason string) {
	if !strings.HasPrefix(reference, "PAYOUT-") {
		w.WriteHeader(http.StatusOK)
		return
	}
	var err error
	switch event {
	case "transfer.success":
		_, err = h.DB.Exec(`UPDATE vendor_payouts SET status = 'paid', paid_at = NOW() WHERE transfer_reference = $1 AND status = 'processing'`, reference)
	case "transfer.failed", "transfer.reversed":
		if reason == "" {
			reason = event
		}
		_, err = h.DB.Exec(`UPDATE vendor_payouts SET status = 'failed', failure_reason = $2 WHERE transfer_reference = $1 AND status <> 'failed'`, reference, reason)
	}
	if err != nil {
		log.Printf("transfer webhook: update failed for %s: %v", reference, err)
		http.Error(w, "retry", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
