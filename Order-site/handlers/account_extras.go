package handlers

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"
)

// accountExtras is the optional part of the account page: wallet, rewards, referral code and
// saved delivery details. It is loaded separately so an older database that lacks one of these
// tables or columns still gets a working account page, just without that card.
type accountExtras struct {
	WalletKobo       int
	LoyaltyPoints    int
	LoyaltyTier      string
	ReferralCode     string
	ReferralCount    int
	AddressLine      string
	State            string
	DeliveryArea     string
	DeliveryNote     string
	PreferredContact string
	MarketingOptIn   bool
	HasWallet        bool
}

func (h *Handlers) loadAccountExtras(ctx context.Context, userID int) accountExtras {
	var x accountExtras
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var code sql.NullString
	err := h.DB.QueryRowContext(ctx, `
		SELECT (SELECT COALESCE(SUM(w.amount_kobo), 0) FROM wallet_transactions w WHERE w.user_id = u.id),
		       (SELECT COALESCE(SUM(CASE WHEN l.type = 'earned' THEN l.points ELSE -l.points END), 0) FROM loyalty_transactions l WHERE l.user_id = u.id),
		       u.referral_code,
		       (SELECT COUNT(*) FROM users r WHERE r.referred_by = u.id),
		       COALESCE(to_jsonb(u)->>'address_line', ''),
		       COALESCE(to_jsonb(u)->>'state', ''),
		       COALESCE(to_jsonb(u)->>'delivery_area', ''),
		       COALESCE(to_jsonb(u)->>'delivery_note', ''),
		       COALESCE(to_jsonb(u)->>'preferred_contact', 'email'),
		       COALESCE((to_jsonb(u)->>'marketing_opt_in')::boolean, FALSE)
		FROM users u WHERE u.id = $1`, userID).Scan(
		&x.WalletKobo, &x.LoyaltyPoints, &code, &x.ReferralCount,
		&x.AddressLine, &x.State, &x.DeliveryArea, &x.DeliveryNote, &x.PreferredContact, &x.MarketingOptIn)
	if err != nil {
		log.Printf("account: extras unavailable for user %d: %v", userID, err)
		return accountExtras{LoyaltyTier: "Bronze", PreferredContact: "email"}
	}
	x.HasWallet = true
	x.ReferralCode = code.String
	x.LoyaltyTier, _, _, _ = loyaltyTier(x.LoyaltyPoints)
	return x
}

// accountCompleteness scores how much of a profile is filled in (0-100) and lists what is
// still missing, so the account page can nudge people towards a profile that makes checkout
// and delivery smoother.
func accountCompleteness(hasName, hasPhoto, hasPhone, hasAddress bool) (percent int, missing []string) {
	steps := []struct {
		done  bool
		label string
	}{
		{hasName, "Add your name"},
		{hasPhoto, "Add a profile photo"},
		{hasPhone, "Verify your phone number"},
		{hasAddress, "Save a delivery address"},
	}
	done := 0
	for _, s := range steps {
		if s.done {
			done++
		} else {
			missing = append(missing, s.label)
		}
	}
	return done * 100 / len(steps), missing
}

func cleanText(s string, max int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}
