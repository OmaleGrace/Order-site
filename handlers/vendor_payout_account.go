package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var nigerianAccountNumber = regexp.MustCompile(`^[0-9]{10}$`)
var payoutHTTPClient = &http.Client{Timeout: 15 * time.Second}

type paystackBankListResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    []struct {
		Name string `json:"name"`
		Code string `json:"code"`
	} `json:"data"`
}

type paystackResolveResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AccountNumber string `json:"account_number"`
		AccountName   string `json:"account_name"`
	} `json:"data"`
}

type payoutJSONResponse struct {
	OK          bool   `json:"ok"`
	Message     string `json:"message,omitempty"`
	AccountName string `json:"account_name,omitempty"`
	Token       string `json:"verification_token,omitempty"`
}

func writePayoutJSON(w http.ResponseWriter, status int, payload payoutJSONResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func randomVerificationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// VendorPayoutBanks returns the current Nigerian bank directory from Paystack.
// The secret key remains on the server; the browser only receives bank names/codes.
func (h *Handlers) VendorPayoutBanks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writePayoutJSON(w, http.StatusMethodNotAllowed, payoutJSONResponse{Message: "Method not allowed"})
		return
	}
	if _, _, ok := h.ownedVendor(w, r); !ok {
		return
	}
	secret := strings.TrimSpace(os.Getenv("PAYSTACK_SECRET_KEY"))
	if secret == "" {
		writePayoutJSON(w, http.StatusServiceUnavailable, payoutJSONResponse{Message: "Bank verification is not configured."})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://api.paystack.co/bank?country=nigeria&perPage=100", nil)
	if err != nil {
		writePayoutJSON(w, http.StatusInternalServerError, payoutJSONResponse{Message: "Could not prepare bank list request."})
		return
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Accept", "application/json")
	resp, err := payoutHTTPClient.Do(req)
	if err != nil {
		log.Printf("payout banks: Paystack request failed: %v", err)
		writePayoutJSON(w, http.StatusBadGateway, payoutJSONResponse{Message: "Paystack could not be reached. Please try again."})
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writePayoutJSON(w, http.StatusBadGateway, payoutJSONResponse{Message: "Could not load the current bank list."})
		return
	}
	var result paystackBankListResponse
	if err := json.Unmarshal(body, &result); err != nil || !result.Status {
		writePayoutJSON(w, http.StatusBadGateway, payoutJSONResponse{Message: "Could not load the current bank list."})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "banks": result.Data})
}

// VendorPayoutAccountVerify resolves the account with Paystack but does not save
// the bank details to the vendor yet. It creates a short-lived server-side token
// that must be presented to the separate Save endpoint.
func (h *Handlers) VendorPayoutAccountVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writePayoutJSON(w, http.StatusMethodNotAllowed, payoutJSONResponse{Message: "Method not allowed"})
		return
	}
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		writePayoutJSON(w, http.StatusBadRequest, payoutJSONResponse{Message: "Invalid form submission."})
		return
	}
	bankCode := strings.TrimSpace(r.FormValue("payout_bank_code"))
	accountNumber := strings.TrimSpace(r.FormValue("payout_account_number"))
	if len(bankCode) < 2 || len(bankCode) > 12 || !nigerianAccountNumber.MatchString(accountNumber) {
		writePayoutJSON(w, http.StatusBadRequest, payoutJSONResponse{Message: "Select a bank and enter a valid 10-digit Nigerian account number."})
		return
	}
	secret := strings.TrimSpace(os.Getenv("PAYSTACK_SECRET_KEY"))
	if secret == "" {
		writePayoutJSON(w, http.StatusServiceUnavailable, payoutJSONResponse{Message: "Bank verification is not configured. Add PAYSTACK_SECRET_KEY in the private environment settings."})
		return
	}

	endpoint := "https://api.paystack.co/bank/resolve?account_number=" + url.QueryEscape(accountNumber) + "&bank_code=" + url.QueryEscape(bankCode)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		writePayoutJSON(w, http.StatusInternalServerError, payoutJSONResponse{Message: "Could not prepare bank verification."})
		return
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Accept", "application/json")
	resp, err := payoutHTTPClient.Do(req)
	if err != nil {
		log.Printf("payout account verification: Paystack request failed: %v", err)
		writePayoutJSON(w, http.StatusBadGateway, payoutJSONResponse{Message: "The bank verification service could not be reached. Please try again later."})
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		writePayoutJSON(w, http.StatusBadGateway, payoutJSONResponse{Message: "Could not read the bank verification response."})
		return
	}
	var result paystackResolveResponse
	if err := json.Unmarshal(body, &result); err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || !result.Status || strings.TrimSpace(result.Data.AccountName) == "" || result.Data.AccountNumber != accountNumber {
		message := "We couldn't verify that account. Check the bank and account number, then try again."
		if result.Message != "" {
			message = result.Message
		}
		writePayoutJSON(w, http.StatusUnprocessableEntity, payoutJSONResponse{Message: message})
		return
	}

	token, err := randomVerificationToken()
	if err != nil {
		writePayoutJSON(w, http.StatusInternalServerError, payoutJSONResponse{Message: "The account was verified, but we could not prepare the save step. Please try again."})
		return
	}
	_, err = h.DB.Exec(`
        INSERT INTO vendor_payout_verifications (token, vendor_id, bank_name, bank_code, account_number, account_name, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW() + INTERVAL '10 minutes')
	`, token, vendorID, strings.TrimSpace(r.FormValue("payout_bank_name")), bankCode, accountNumber, strings.TrimSpace(result.Data.AccountName))
	if err != nil {
		log.Printf("payout account verification: pending verification insert failed: %v", err)
		writePayoutJSON(w, http.StatusInternalServerError, payoutJSONResponse{Message: "The account was verified, but we could not prepare the save step."})
		return
	}
	writePayoutJSON(w, http.StatusOK, payoutJSONResponse{OK: true, Message: "Account verified. Review the account name before saving.", AccountName: strings.TrimSpace(result.Data.AccountName), Token: token})
}

// VendorPayoutAccountSave saves only a Paystack-verified account represented by
// a short-lived, server-side verification token.
func (h *Handlers) VendorPayoutAccountSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writePayoutJSON(w, http.StatusMethodNotAllowed, payoutJSONResponse{Message: "Method not allowed"})
		return
	}
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		writePayoutJSON(w, http.StatusBadRequest, payoutJSONResponse{Message: "Invalid form submission."})
		return
	}
	token := strings.TrimSpace(r.FormValue("verification_token"))
	if len(token) != 64 {
		writePayoutJSON(w, http.StatusBadRequest, payoutJSONResponse{Message: "Verify the account first."})
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writePayoutJSON(w, http.StatusInternalServerError, payoutJSONResponse{Message: "Could not save bank details."})
		return
	}
	defer tx.Rollback()

	var bankName, bankCode, accountNumber, accountName string
	err = tx.QueryRow(`
    SELECT bank_name, bank_code, account_number, account_name
    FROM vendor_payout_verifications
    WHERE token = $1 AND vendor_id = $2 AND used_at IS NULL AND expires_at > NOW()
    FOR UPDATE
`, token, vendorID).Scan(
		&bankName,
		&bankCode,
		&accountNumber,
		&accountName,
	)
	if err == sql.ErrNoRows {
		writePayoutJSON(w, http.StatusBadRequest, payoutJSONResponse{Message: "This verification has expired. Verify the account again."})
		return
	}
	if err != nil {
		log.Printf("payout account save: token lookup failed: %v", err)
		writePayoutJSON(w, http.StatusInternalServerError, payoutJSONResponse{Message: "Could not verify the pending bank details."})
		return
	}

	_, err = tx.Exec(`
        UPDATE vendors
        SET payout_bank_name=$1, payout_bank_code=$2, payout_account_number=$3,
            payout_account_name=$4, payout_account_verified=TRUE, payout_account_verified_at=NOW()
        WHERE id=$5
    `, bankName, bankCode, accountNumber, accountName, vendorID)
	if err != nil {
		log.Printf("payout account save: vendor update failed: %v", err)
		writePayoutJSON(w, http.StatusInternalServerError, payoutJSONResponse{Message: "Could not save the verified bank account."})
		return
	}
	if _, err = tx.Exec(`UPDATE vendor_payout_verifications SET used_at=NOW() WHERE token=$1`, token); err != nil {
		log.Printf("payout account save: token update failed: %v", err)
		writePayoutJSON(w, http.StatusInternalServerError, payoutJSONResponse{Message: "Could not finish saving the verified bank account."})
		return
	}
	if _, err = tx.Exec(`DELETE FROM vendor_payout_verifications WHERE vendor_id=$1 AND (used_at IS NOT NULL OR expires_at <= NOW())`, vendorID); err != nil {
		log.Printf("payout account save: cleanup failed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		writePayoutJSON(w, http.StatusInternalServerError, payoutJSONResponse{Message: "Could not finish saving the verified bank account."})
		return
	}
	writePayoutJSON(w, http.StatusOK, payoutJSONResponse{OK: true, Message: fmt.Sprintf("Bank account saved and verified for %s.", accountName)})
}
