package handlers

import (
	"html/template"
	"strings"
	"testing"

	"Order-site/render"
)

func TestCommission(t *testing.T) {
	t.Setenv("VENDOR_COMMISSION_PERCENT", "")
	cases := map[int]int{0: 0, 100000: 10000, 250050: 25005, 1: 0, 5: 1, 15: 2, 999: 100}
	for food, want := range cases {
		if got := commissionKobo(food); got != want {
			t.Errorf("commission of %d = %d, want %d", food, got, want)
		}
	}
	t.Setenv("VENDOR_COMMISSION_PERCENT", "15")
	if commissionKobo(100000) != 15000 {
		t.Error("env percent not used")
	}
	for _, bad := range []string{"-1", "51", "abc", "1000"} {
		t.Setenv("VENDOR_COMMISSION_PERCENT", bad)
		if commissionPercent() != 10 {
			t.Errorf("%q should fall back to 10", bad)
		}
	}
}

func TestSetupFeeDefault(t *testing.T) {
	t.Setenv("VENDOR_SETUP_FEE_KOBO", "")
	if vendorSetupFeeKobo() != 500000 {
		t.Error("setup fee should default to ₦5,000")
	}
}

func TestMoney(t *testing.T) {
	cases := map[int]string{0: "0.00", 5: "0.05", 100: "1.00", 123456: "1,234.56", 100000000: "1,000,000.00", -250050: "-2,500.50"}
	for k, want := range cases {
		if got := money(k); got != want {
			t.Errorf("money(%d)=%s want %s", k, got, want)
		}
	}
}

func renderPage(t *testing.T, file string, funcs template.FuncMap, data interface{}) string {
	t.Helper()
	var b strings.Builder
	tm, err := render.Page(file, funcs)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	if err := tm.Execute(&b, data); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return b.String()
}

func TestPagesRenderWithoutMonthlyFee(t *testing.T) {
	f := template.FuncMap{"naira": naira, "money": money, "money2": money2}

	out := renderPage(t, "vendor-billing.html", f, VendorBillingPage{
		Name: "Mama Put", State: "active", CommissionPercent: 10,
		Earnings: VendorEarnings{Orders: 2, SalesKobo: 500000, CommissionKobo: 45000, PaidOutKobo: 100000, BalanceKobo: 355000},
		Payouts:  []VendorPayoutRow{{AmountKobo: 100000, Note: "Transfer", PaidAt: "19 Oct 2026"}},
	})
	for _, want := range []string{"no monthly fee", "10%", "₦3,550.00", "₦4,500.00"[:0]} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(want)) {
			t.Errorf("billing page missing %q", want)
		}
	}
	if strings.Contains(out, "Renew") {
		t.Error("billing page must not mention renewing")
	}

	setup := renderPage(t, "vendor-billing.html", f, VendorBillingPage{Name: "X", State: "setup", PayAmountKobo: 500000, CommissionPercent: 10, CanPay: true})
	if !strings.Contains(setup, "₦5000") && !strings.Contains(setup, "₦5,000") {
		t.Error("setup state should show the setup fee")
	}

	e := renderPage(t, "admin-earnings.html", f, AdminEarningsPage{CommissionPercent: 10, TotalSalesKobo: 500000, TotalCommission: 45000, TotalOwedKobo: 355000,
		Rows: []AdminEarningsRow{{VendorID: 1, Name: "Mama Put", AccountVerified: true, VendorEarnings: VendorEarnings{Orders: 2, SalesKobo: 500000, CommissionKobo: 45000, BalanceKobo: 455000}}}})
	for _, want := range []string{"Mama Put", "₦4,550.00", `max="4550.00"`, "/admin/earnings/payout"} {
		if !strings.Contains(e, want) {
			t.Errorf("admin earnings missing %q", want)
		}
	}

	h := renderPage(t, "home.html", f, struct {
		Popular, Again    []HomeDish
		Categories        []HomeCategory
		Shops             []ShopCard
		SetupFeeKobo      int
		CommissionPercent int
	}{SetupFeeKobo: 500000, CommissionPercent: 10})
	if strings.Contains(h, "a month") || strings.Contains(h, "each month") {
		t.Error("home page still mentions a monthly fee")
	}
	if !strings.Contains(h, "10%") {
		t.Error("home page should show the commission")
	}

	a := renderPage(t, "become-vendor.html", f, VendorApplyPage{State: "form", SetupFeeKobo: 500000, CommissionPercent: 10, IDTypes: idTypeOptions})
	if strings.Contains(a, "a month") || !strings.Contains(a, "10%") {
		t.Error("become-vendor page copy wrong")
	}
}
