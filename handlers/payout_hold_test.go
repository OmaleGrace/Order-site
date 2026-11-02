package handlers

import "testing"

func TestSettlePosition(t *testing.T) {
	cases := []struct {
		name                                       string
		avail, held, inProgress, paid              int
		wantAvail, wantHeld, wantProg, wantPaidOut int
	}{
		{"nothing paid yet", 92000, 46000, 27600, 0, 92000, 46000, 27600, 0},
		{"payout comes out of available first", 92000, 46000, 27600, 12000, 80000, 46000, 27600, 12000},
		{"overpaid early eats into held, then in progress", 10000, 5000, 5000, 17000, 0, 0, 3000, 17000},
		{"paid everything", 100, 0, 0, 100, 0, 0, 0, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := settlePosition(c.avail, c.held, c.inProgress, c.paid)
			if p.AvailableKobo != c.wantAvail || p.HeldKobo != c.wantHeld || p.InProgressKobo != c.wantProg || p.PaidKobo != c.wantPaidOut {
				t.Fatalf("got %+v", p)
			}
		})
	}
}

func TestPayoutHoldHoursDefault(t *testing.T) {
	t.Setenv("PAYOUT_HOLD_HOURS", "")
	if payoutHoldHours() != 24 {
		t.Fatal("default hold should be 24 hours")
	}
	t.Setenv("PAYOUT_HOLD_HOURS", "48")
	if payoutHoldHours() != 48 {
		t.Fatal("PAYOUT_HOLD_HOURS should be used")
	}
	t.Setenv("PAYOUT_HOLD_HOURS", "9999")
	if payoutHoldHours() != 24 {
		t.Fatal("out-of-range values fall back to 24")
	}
}
