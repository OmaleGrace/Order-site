package handlers

import (
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

	"Order-site/errors"
)

var nigerianAccountNumber = regexp.MustCompile(`^[0-9]{10}$`)
var payoutHTTPClient = &http.Client{Timeout: 15 * time.Second}

type paystackBankListResponse struct {
	Status bool `json:"status"`
	Data   []struct {
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

// VendorPayoutAccountVerify verifies a Nigerian bank account using Paystack's
// account-resolution endpoint and stores it privately for administrator payouts.
func (h *Handlers) VendorPayoutAccountVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	back := func(msg string) {
		http.Redirect(w, r, "/vendor/profile?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Invalid form submission", http.StatusBadRequest)
		return
	}
	bankName := strings.TrimSpace(r.FormValue("payout_bank_name"))
	bankCode := strings.TrimSpace(r.FormValue("payout_bank_code"))
	accountNumber := strings.TrimSpace(r.FormValue("payout_account_number"))
	if len([]rune(bankName)) < 2 || len([]rune(bankName)) > 100 || len(bankCode) < 2 || len(bankCode) > 12 || !nigerianAccountNumber.MatchString(accountNumber) {
		back("Enter a bank name, valid bank code, and a 10-digit Nigerian account number.")
		return
	}
	secret := strings.TrimSpace(os.Getenv("PAYSTACK_SECRET_KEY"))
	if secret == "" {
		back("Bank verification is not configured. Add PAYSTACK_SECRET_KEY in your private environment settings, then try again.")
		return
	}
	// Confirm that the supplied bank code and bank name refer to the same
	// bank in Paystack's Nigerian bank directory before resolving the account.
	bankReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://api.paystack.co/bank?country=nigeria&perPage=200", nil)
	if err != nil {
		back("Could not prepare bank verification. Please try again.")
		return
	}
	bankReq.Header.Set("Authorization", "Bearer "+secret)
	bankResp, err := payoutHTTPClient.Do(bankReq)
	if err != nil {
		log.Printf("payout account verification: bank list request failed: %v", err)
		back("The bank verification service could not be reached. Please try again later.")
		return
	}
	bankBody, readErr := io.ReadAll(io.LimitReader(bankResp.Body, 1<<20))
	bankResp.Body.Close()
	var bankList paystackBankListResponse
	if readErr != nil || bankResp.StatusCode < 200 || bankResp.StatusCode >= 300 || json.Unmarshal(bankBody, &bankList) != nil || !bankList.Status {
		back("Could not confirm the bank details with Paystack. Please try again.")
		return
	}
	bankMatches := false
	for _, bank := range bankList.Data {
		if strings.EqualFold(strings.TrimSpace(bank.Code), bankCode) && strings.EqualFold(strings.TrimSpace(bank.Name), bankName) {
			bankMatches = true
			bankName = strings.TrimSpace(bank.Name)
			bankCode = strings.TrimSpace(bank.Code)
			break
		}
	}
	if !bankMatches {
		back("The bank name and bank code do not match Paystack's bank directory. Check both and try again.")
		return
	}

	endpoint := "https://api.paystack.co/bank/resolve?account_number=" + url.QueryEscape(accountNumber) + "&bank_code=" + url.QueryEscape(bankCode)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		back("Could not prepare bank verification. Please try again.")
		return
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Accept", "application/json")
	resp, err := payoutHTTPClient.Do(req)
	if err != nil {
		log.Printf("payout account verification: Paystack request failed: %v", err)
		back("The bank verification service could not be reached. Please try again later.")
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		back("Could not read the bank verification response. Please try again.")
		return
	}
	var result paystackResolveResponse
	if err := json.Unmarshal(body, &result); err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || !result.Status || strings.TrimSpace(result.Data.AccountName) == "" || result.Data.AccountNumber != accountNumber {
		back("We couldn't verify that account. Check the bank code and account number, then try again.")
		return
	}
	_, err = h.DB.Exec(`UPDATE vendors SET payout_bank_name=$1, payout_bank_code=$2, payout_account_number=$3, payout_account_name=$4, payout_account_verified=TRUE, payout_account_verified_at=NOW() WHERE id=$5`, bankName, bankCode, accountNumber, strings.TrimSpace(result.Data.AccountName), vendorID)
	if err != nil {
		log.Printf("payout account verification: database update failed: %v", err)
		back("The account was verified but could not be saved. Please try again.")
		return
	}
	back(fmt.Sprintf("Bank account verified for %s. Only the platform administrator can view the full account number.", strings.TrimSpace(result.Data.AccountName)))
}
