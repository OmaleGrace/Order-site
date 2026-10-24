package handlers

import (
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
