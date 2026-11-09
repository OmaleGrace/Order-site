package handlers

import "testing"

func TestNewReferralCodeMatchesPattern(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := newReferralCode()
		if err != nil {
			t.Fatal(err)
		}
		if !referralCodePattern.MatchString(code) {
			t.Fatalf("code %q does not match the accepted pattern", code)
		}
	}
}

func TestReferralDefaults(t *testing.T) {
	t.Setenv("REFERRAL_BONUS_POINTS", "")
	t.Setenv("REFERRAL_MIN_ORDER_KOBO", "")
	if referralBonusPoints() != 300 || referralMinOrderKobo() != 150000 {
		t.Fatalf("unexpected defaults: %d, %d", referralBonusPoints(), referralMinOrderKobo())
	}
}
