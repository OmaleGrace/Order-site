package handlers

import (
	"database/sql"
	"os"
	"strconv"
)

func envKobo(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

// One-time setup fee. Default ₦5,000. There is no monthly fee.
func vendorSetupFeeKobo() int { return envKobo("VENDOR_SETUP_FEE_KOBO", 500000) }

// commissionPercent is the share of each food sale that the platform keeps.
// Default 10. Set VENDOR_COMMISSION_PERCENT to change it (0 to 50).
func commissionPercent() int {
	if v, err := strconv.Atoi(os.Getenv("VENDOR_COMMISSION_PERCENT")); err == nil && v >= 0 && v <= 50 {
		return v
	}
	return 10
}

// commissionKobo is the platform's cut of the food total (delivery fees are not included),
// rounded to the nearest kobo.
func commissionKobo(foodKobo int) int {
	return (foodKobo*commissionPercent() + 50) / 100
}

// proCommissionPercent is the lower commission a vendor on the paid Pro plan pays.
// Default 5. Set PRO_COMMISSION_PERCENT to change it (0 to 50).
func proCommissionPercent() int {
	if v, err := strconv.Atoi(os.Getenv("PRO_COMMISSION_PERCENT")); err == nil && v >= 0 && v <= 50 {
		return v
	}
	return 5
}

// vendorTrialDays is how many days a new shop pays no commission. Default 0 (off).
// Set VENDOR_TRIAL_DAYS, for example 30, to give new vendors a risk-free first month.
func vendorTrialDays() int {
	if v, err := strconv.Atoi(os.Getenv("VENDOR_TRIAL_DAYS")); err == nil && v > 0 && v <= 365 {
		return v
	}
	return 0
}

// effectiveCommissionPercent is the rate this vendor pays right now: nothing during the
// free trial, the lower rate on an active Pro plan, otherwise the normal rate.
func effectiveCommissionPercent(db *sql.DB, vendorID int) int {
	percent := commissionPercent()
	var inTrial, onPro bool
	err := db.QueryRow(`
		SELECT ($2 > 0 AND created_at > NOW() - make_interval(days => $2)),
		       (plan = 'pro' AND plan_until IS NOT NULL AND plan_until > NOW())
		FROM vendors WHERE id = $1`, vendorID, vendorTrialDays()).Scan(&inTrial, &onPro)
	if err == nil {
		switch {
		case inTrial:
			percent = 0
		case onPro && proCommissionPercent() < percent:
			percent = proCommissionPercent()
		}
	}
	return percent
}

// commissionKoboFor is the platform's cut for one vendor's order. It is stored on the order
// when it is placed, so later changes never rewrite past orders.
func commissionKoboFor(db *sql.DB, vendorID, foodKobo int) int {
	return (foodKobo*effectiveCommissionPercent(db, vendorID) + 50) / 100
}

// Prices for the self-serve extras. Pro is per 30 days; promotion is per 7 days.
func proPlanPriceKobo() int       { return envKobo("PRO_PLAN_PRICE_KOBO", 500000) }
func promotionWeekPriceKobo() int { return envKobo("PROMOTION_WEEK_PRICE_KOBO", 300000) }

// serviceFeeKobo is the flat fee added to every customer's order and kept by the platform.
// Default 0 (no fee). Set SERVICE_FEE_KOBO, for example 10000 for N100. Capped at N1,000.
func serviceFeeKobo() int {
	if v, err := strconv.Atoi(os.Getenv("SERVICE_FEE_KOBO")); err == nil && v > 0 && v <= 100000 {
		return v
	}
	return 0
}
