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

// One-time setup fee (includes the first month). Default ₦5,000.
func vendorSetupFeeKobo() int { return envKobo("VENDOR_SETUP_FEE_KOBO", 500000) }

// Monthly renewal fee. Default ₦4,000.
func vendorMonthlyFeeKobo() int { return envKobo("VENDOR_MONTHLY_FEE_KOBO", 400000) }
