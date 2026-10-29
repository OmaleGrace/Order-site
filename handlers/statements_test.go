package handlers

import (
	"net/url"
	"testing"
	"time"
)

// 29 Oct 2026, 10:00 in Nigeria (09:00 UTC).
var statementTestNow = time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)

func utcText(t time.Time) string { return t.UTC().Format("2006-01-02 15:04") }

func TestParseStatementPeriodPresets(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		now       time.Time
		wantStart string // UTC; Nigerian midnight is 23:00 the evening before
		wantEnd   string
		wantLabel string
		wantKey   string
	}{
		{"default is this month", "", statementTestNow, "2026-09-30 23:00", "2026-10-31 23:00", "October 2026", "this-month"},
		{"this month", "period=this-month", statementTestNow, "2026-09-30 23:00", "2026-10-31 23:00", "October 2026", "this-month"},
		{"last month", "period=last-month", statementTestNow, "2026-08-31 23:00", "2026-09-30 23:00", "September 2026", "last-month"},
		{"last 3 months", "period=last-3-months", statementTestNow, "2026-07-31 23:00", "2026-10-31 23:00", "Aug 2026 to Oct 2026", "last-3-months"},
		{"this year", "period=this-year", statementTestNow, "2025-12-31 23:00", "2026-12-31 23:00", "2026", "this-year"},
		{"unknown preset falls back to this month", "period=nonsense", statementTestNow, "2026-09-30 23:00", "2026-10-31 23:00", "October 2026", "this-month"},
		{"last month across new year", "period=last-month", time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC), "2025-11-30 23:00", "2025-12-31 23:00", "December 2025", "last-month"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, _ := url.ParseQuery(tc.query)
			p, problem := parseStatementPeriod(q, tc.now)
			if problem != "" {
				t.Fatalf("unexpected problem: %q", problem)
			}
			if got := utcText(p.Start); got != tc.wantStart {
				t.Errorf("start = %s, want %s", got, tc.wantStart)
			}
			if got := utcText(p.End); got != tc.wantEnd {
				t.Errorf("end = %s, want %s", got, tc.wantEnd)
			}
			if p.Label != tc.wantLabel {
				t.Errorf("label = %q, want %q", p.Label, tc.wantLabel)
			}
			if p.Preset != tc.wantKey {
				t.Errorf("preset = %q, want %q", p.Preset, tc.wantKey)
			}
		})
	}
}

func TestParseStatementPeriodAllTimeIncludesToday(t *testing.T) {
	q, _ := url.ParseQuery("period=all")
	p, _ := parseStatementPeriod(q, statementTestNow)
	if got := utcText(p.End); got != "2026-10-29 23:00" {
		t.Errorf("all-time end = %s, want the end of today in Nigeria (2026-10-29 23:00 UTC)", got)
	}
	if !p.End.After(statementTestNow) {
		t.Error("all time must reach past the current moment")
	}
}

func TestParseStatementPeriodCustomDatesAreInclusive(t *testing.T) {
	q, _ := url.ParseQuery("from=2026-10-05&to=2026-10-07")
	p, problem := parseStatementPeriod(q, statementTestNow)
	if problem != "" {
		t.Fatalf("unexpected problem: %q", problem)
	}
	if utcText(p.Start) != "2026-10-04 23:00" || utcText(p.End) != "2026-10-07 23:00" {
		t.Errorf("range = %s to %s, want 5 Oct through the whole of 7 Oct", utcText(p.Start), utcText(p.End))
	}
	if p.Label != "5 Oct 2026 to 7 Oct 2026" || p.Preset != "custom" {
		t.Errorf("label %q preset %q", p.Label, p.Preset)
	}
}

func TestParseStatementPeriodRejectsBadDates(t *testing.T) {
	for _, raw := range []string{
		"from=2026-10-07&to=2026-10-05", // end before start
		"from=2026-10-05",               // missing end
		"from=not-a-date&to=2026-10-05",
		"from=2020-01-01&to=2026-10-05", // longer than three years
	} {
		q, _ := url.ParseQuery(raw)
		p, problem := parseStatementPeriod(q, statementTestNow)
		if problem == "" {
			t.Errorf("%q: expected a problem message", raw)
		}
		if p.Preset != "this-month" {
			t.Errorf("%q: should fall back to this month, got %q", raw, p.Preset)
		}
	}
}

func TestNewStatementOrderAddsUp(t *testing.T) {
	// Food 5000 - coupon 500 - loyalty 200 (200 points) = 4300; delivery 800; paid 5100; commission 10% of 4300 = 430.
	o := newStatementOrder(7, statementTestNow, "delivery", "completed", 510000, 80000, 43000, 200, 50000)
	if o.FoodKobo != 500000 {
		t.Errorf("food = %d, want 500000", o.FoodKobo)
	}
	if o.LoyaltyKobo != 20000 {
		t.Errorf("loyalty = %d, want 20000", o.LoyaltyKobo)
	}
	if o.FoodKobo-o.CouponKobo-o.LoyaltyKobo+o.DeliveryKobo != o.CollectedKobo {
		t.Errorf("food - coupon - loyalty + delivery = %d, want collected %d",
			o.FoodKobo-o.CouponKobo-o.LoyaltyKobo+o.DeliveryKobo, o.CollectedKobo)
	}
	if o.NetKobo != 467000 {
		t.Errorf("net = %d, want 467000", o.NetKobo)
	}
	if o.CreatedAt.Location() != watZone {
		t.Error("order times should be shown in Nigerian time")
	}
}

func TestSummarizeStatement(t *testing.T) {
	orders := []StatementOrder{
		newStatementOrder(1, statementTestNow, "pickup", "paid", 100000, 0, 10000, 0, 0),
		newStatementOrder(2, statementTestNow, "delivery", "completed", 150000, 50000, 10000, 0, 0),
	}
	got := summarizeStatement(orders, 60000, 25000)
	if got.Orders != 2 || got.CollectedKobo != 250000 || got.CommissionKobo != 20000 || got.NetKobo != 230000 {
		t.Errorf("totals wrong: %+v", got)
	}
	if got.DeliveryKobo != 50000 || got.FoodKobo != 200000 {
		t.Errorf("food/delivery wrong: %+v", got)
	}
	if want := 25000 + 230000 - 60000; got.ClosingKobo != want {
		t.Errorf("closing = %d, want %d", got.ClosingKobo, want)
	}
}

func TestSummarizeStatementEmptyPeriodKeepsBalance(t *testing.T) {
	got := summarizeStatement(nil, 0, 123400)
	if got.ClosingKobo != 123400 || got.Orders != 0 {
		t.Errorf("a quiet period should carry the balance forward unchanged: %+v", got)
	}
}

func TestFormatNaira2AndCSVMoney(t *testing.T) {
	if got := formatNaira2(125050); got != "₦1,250.50" {
		t.Errorf("formatNaira2 = %q", got)
	}
	if got := formatNaira2(-125050); got != "-₦1,250.50" {
		t.Errorf("negative formatNaira2 = %q", got)
	}
	if got := csvMoney(125050); got != "1250.50" {
		t.Errorf("csvMoney = %q", got)
	}
	if got := csvMoney(-5); got != "-0.05" {
		t.Errorf("negative csvMoney = %q", got)
	}
}

func TestCSVCellBlocksFormulas(t *testing.T) {
	for in, want := range map[string]string{
		"Mama Put":        "Mama Put",
		"":                "",
		"=SUM(A1:A9)":     "'=SUM(A1:A9)",
		"+234 call me":    "'+234 call me",
		"-cmd":            "'-cmd",
		"@import":         "'@import",
		"Paid in October": "Paid in October",
	} {
		if got := csvCell(in); got != want {
			t.Errorf("csvCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStatementFilename(t *testing.T) {
	q, _ := url.ParseQuery("from=2026-10-05&to=2026-10-07")
	p, _ := parseStatementPeriod(q, statementTestNow)
	if got := statementFilename("enie-statement", p); got != "enie-statement-2026-10-05-to-2026-10-07.csv" {
		t.Errorf("filename = %q", got)
	}
}

func TestSumSettlements(t *testing.T) {
	rows := []AdminSettlementRow{
		{Orders: 2, CollectedKobo: 250000, DeliveryKobo: 50000, CommissionKobo: 20000, NetKobo: 230000, PaidInKobo: 60000, OutstandingKobo: 170000},
		{Orders: 1, CollectedKobo: 100000, CommissionKobo: 10000, NetKobo: 90000, OutstandingKobo: 90000},
	}
	got := sumSettlements(rows)
	if got.Orders != 3 || got.CollectedKobo != 350000 || got.CommissionKobo != 30000 || got.NetKobo != 320000 ||
		got.PaidInKobo != 60000 || got.OutstandingKobo != 260000 || got.DeliveryKobo != 50000 {
		t.Errorf("totals wrong: %+v", got)
	}
}
