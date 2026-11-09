package handlers

import (
	"testing"
	"time"
)

// at builds a Lagos-time moment. 2 Nov 2026 is a Monday.
func at(day, hour, min int) time.Time {
	return time.Date(2026, time.November, day, hour, min, 0, 0, lagosLoc)
}

func TestOpenStatus(t *testing.T) {
	var daytime weekHours
	for _, wd := range []int{1, 2, 3, 4, 5} { // Mon–Fri
		daytime[wd] = &dayHours{"09:00", "21:00"}
	}
	var night weekHours
	night[5] = &dayHours{"18:00", "02:00"} // Friday evening into Saturday

	cases := []struct {
		name      string
		w         weekHours
		now       time.Time
		known     bool
		open      bool
		wantLabel string
	}{
		{"no hours set", weekHours{}, at(2, 12, 0), false, false, ""},
		{"open midday Monday", daytime, at(2, 12, 0), true, true, "Open now · closes 9:00 PM"},
		{"closed early Monday", daytime, at(2, 7, 30), true, false, "Closed · opens 9:00 AM"},
		{"closed Monday night", daytime, at(2, 22, 0), true, false, "Closed · opens tomorrow 9:00 AM"},
		{"closed Friday night opens Monday", daytime, at(6, 22, 0), true, false, "Closed · opens Mon 9:00 AM"},
		{"closing time itself is closed", daytime, at(2, 21, 0), true, false, "Closed · opens tomorrow 9:00 AM"},
		{"overnight open Friday evening", night, at(6, 23, 0), true, true, "Open now · closes 2:00 AM"},
		{"overnight still open Saturday 1am", night, at(7, 1, 0), true, true, "Open now · closes 2:00 AM"},
		{"overnight closed Saturday 3am", night, at(7, 3, 0), true, false, "Closed · opens Fri 6:00 PM"},
	}
	for _, c := range cases {
		known, open, label := openStatus(c.w, c.now)
		if known != c.known || open != c.open || label != c.wantLabel {
			t.Errorf("%s: got (%v, %v, %q), want (%v, %v, %q)", c.name, known, open, label, c.known, c.open, c.wantLabel)
		}
	}
}
