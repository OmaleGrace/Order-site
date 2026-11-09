package handlers

import (
	"strings"
	"testing"
)

func TestApplyDeliveryRate(t *testing.T) {
	fiveHundred := 50000 // ₦500 in kobo
	cases := []struct {
		name        string
		rate        *CheckoutRate
		foodKobo    int
		defaultFee  int
		wantFee     int
		wantBlocked bool
	}{
		{"unpriced area uses the shop default", nil, 300000, 80000, 80000, false},
		{"area fee replaces the default", &CheckoutRate{LGA: "Ikeja", FeeKobo: &fiveHundred}, 300000, 80000, 50000, false},
		{"blank area fee keeps the default", &CheckoutRate{LGA: "Ikeja"}, 300000, 80000, 80000, false},
		{"reaching the free-delivery amount makes it free", &CheckoutRate{LGA: "Ikeja", FeeKobo: &fiveHundred, FreeOverKobo: 500000}, 500000, 80000, 0, false},
		{"one kobo under the free amount still pays", &CheckoutRate{LGA: "Ikeja", FeeKobo: &fiveHundred, FreeOverKobo: 500000}, 499999, 80000, 50000, false},
		{"free delivery also applies to the default fee", &CheckoutRate{LGA: "Ikeja", FreeOverKobo: 500000}, 600000, 80000, 0, false},
		{"below the minimum order is blocked", &CheckoutRate{LGA: "Ikeja", MinOrderKobo: 200000}, 150000, 80000, 80000, true},
		{"meeting the minimum order is allowed", &CheckoutRate{LGA: "Ikeja", MinOrderKobo: 200000}, 200000, 80000, 80000, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fee, blocked := applyDeliveryRate(tc.rate, tc.foodKobo, tc.defaultFee)
			if fee != tc.wantFee {
				t.Errorf("fee = %d, want %d", fee, tc.wantFee)
			}
			if (blocked != "") != tc.wantBlocked {
				t.Errorf("blocked = %q, wantBlocked %v", blocked, tc.wantBlocked)
			}
		})
	}
}

func TestBlockedMessageIsFriendly(t *testing.T) {
	_, blocked := applyDeliveryRate(&CheckoutRate{LGA: "Ikeja", MinOrderKobo: 250000}, 100000, 50000)
	for _, want := range []string{"Ikeja", "₦2,500", "pickup"} {
		if !strings.Contains(blocked, want) {
			t.Errorf("message %q should mention %q", blocked, want)
		}
	}
}

func TestFormatNaira(t *testing.T) {
	cases := map[int]string{0: "₦0", 99: "₦0", 50000: "₦500", 1250000: "₦12,500", 100000000: "₦1,000,000"}
	for kobo, want := range cases {
		if got := formatNaira(kobo); got != want {
			t.Errorf("formatNaira(%d) = %q, want %q", kobo, got, want)
		}
	}
}

func TestNairaText(t *testing.T) {
	if got := nairaText(150000); got != "1500" {
		t.Errorf("nairaText(150000) = %q", got)
	}
	if got := nairaText(150050); got != "1500.50" {
		t.Errorf("nairaText(150050) = %q", got)
	}
}

func TestNairaToKobo(t *testing.T) {
	cases := []struct {
		in        string
		wantKobo  int
		wantBlank bool
		wantOK    bool
	}{
		{"1500", 150000, false, true},
		{"1,500.50", 150050, false, true},
		{"₦200", 20000, false, true},
		{"  75  ", 7500, false, true},
		{"0", 0, false, true},
		{"", 0, true, true},
		{"   ", 0, true, true},
		{"-5", 0, false, false},
		{"abc", 0, false, false},
		{"NaN", 0, false, false},
		{"Inf", 0, false, false},
		{"100001", 0, false, false}, // above the ₦100,000 limit used in these cases
	}
	for _, tc := range cases {
		kobo, blank, ok := nairaToKobo(tc.in, 100000)
		if kobo != tc.wantKobo || blank != tc.wantBlank || ok != tc.wantOK {
			t.Errorf("nairaToKobo(%q) = (%d, %v, %v), want (%d, %v, %v)",
				tc.in, kobo, blank, ok, tc.wantKobo, tc.wantBlank, tc.wantOK)
		}
	}
}

func TestAreaKeyIgnoresCaseAndSpacing(t *testing.T) {
	if areaKey(" Lagos ", "IKEJA") != areaKey("lagos", " ikeja ") {
		t.Error("areaKey should ignore case and surrounding spaces")
	}
	if areaKey("Lagos", "Ikeja") == areaKey("Lagos", "Surulere") {
		t.Error("different areas must have different keys")
	}
}
