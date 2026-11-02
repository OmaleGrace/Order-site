package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lib/pq"
)

// dayHours is one day's opening window as "HH:MM" text. A day with no dayHours is a closed day.
type dayHours struct{ Opens, Closes string }

// weekHours holds a week of opening windows, indexed by time.Weekday (Sunday is 0).
// A vendor with no days set at all has not published opening hours, so no status is shown for them.
type weekHours [7]*dayHours

func (w weekHours) isSet() bool {
	for _, d := range w {
		if d != nil {
			return true
		}
	}
	return false
}

var weekdayNames = [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// weekdayOrder is the order days are shown in forms and lists (Monday first).
var weekdayOrder = [7]int{1, 2, 3, 4, 5, 6, 0}

// lagosLoc is the time zone opening hours are read in. Nigeria has no daylight saving, so if the
// server has no tz database we fall back to a fixed UTC+1.
var lagosLoc = func() *time.Location {
	if loc, err := time.LoadLocation("Africa/Lagos"); err == nil {
		return loc
	}
	return time.FixedZone("WAT", 3600)
}()

func clockMinutes(s string) (int, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// clock12 turns "14:30" into "2:30 PM".
func clock12(s string) string {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return s
	}
	return t.Format("3:04 PM")
}

// window returns a day's opening and closing minute and whether the window runs past midnight.
func (d *dayHours) window() (open, closeAt int, overnight, ok bool) {
	if d == nil {
		return 0, 0, false, false
	}
	o, ok1 := clockMinutes(d.Opens)
	c, ok2 := clockMinutes(d.Closes)
	if !ok1 || !ok2 || o == c {
		return 0, 0, false, false
	}
	return o, c, c < o, true
}

// openStatus says whether a shop is open at `now` and gives a short label for it. known is false
// when the vendor has not set any hours.
func openStatus(w weekHours, now time.Time) (known, open bool, label string) {
	if !w.isSet() {
		return false, false, ""
	}
	now = now.In(lagosLoc)
	cur := int(now.Weekday())
	mins := now.Hour()*60 + now.Minute()

	if o, c, overnight, ok := w[cur].window(); ok {
		if (!overnight && mins >= o && mins < c) || (overnight && mins >= o) {
			return true, true, "Open now · closes " + clock12(w[cur].Closes)
		}
	}
	// A window that started yesterday evening and runs past midnight.
	if _, c, overnight, ok := w[(cur+6)%7].window(); ok && overnight && mins < c {
		return true, true, "Open now · closes " + clock12(w[(cur+6)%7].Closes)
	}

	// Closed: say when it opens next.
	if o, _, _, ok := w[cur].window(); ok && mins < o {
		return true, false, "Closed · opens " + clock12(w[cur].Opens)
	}
	for i := 1; i <= 7; i++ {
		day := (cur + i) % 7
		if _, _, _, ok := w[day].window(); !ok {
			continue
		}
		when := weekdayNames[day][:3]
		if i == 1 {
			when = "tomorrow"
		}
		return true, false, "Closed · opens " + when + " " + clock12(w[day].Opens)
	}
	return true, false, "Closed"
}

// ShopHoursRow is one line of the weekly opening hours list on a shop page.
type ShopHoursRow struct {
	Day  string
	Text string
}

func hoursRows(w weekHours) []ShopHoursRow {
	if !w.isSet() {
		return nil
	}
	rows := make([]ShopHoursRow, 0, 7)
	for _, wd := range weekdayOrder {
		text := "Closed"
		if d := w[wd]; d != nil {
			text = clock12(d.Opens) + " – " + clock12(d.Closes)
		}
		rows = append(rows, ShopHoursRow{Day: weekdayNames[wd], Text: text})
	}
	return rows
}

// vendorWeekHours loads one vendor's saved opening hours.
func (h *Handlers) vendorWeekHours(vendorID int) (weekHours, error) {
	var w weekHours
	rows, err := h.DB.Query(`SELECT weekday, opens, closes FROM vendor_hours WHERE vendor_id = $1`, vendorID)
	if err != nil {
		return w, err
	}
	defer rows.Close()
	for rows.Next() {
		var wd int
		var d dayHours
		if err := rows.Scan(&wd, &d.Opens, &d.Closes); err != nil {
			return w, err
		}
		if wd >= 0 && wd < 7 {
			w[wd] = &d
		}
	}
	return w, rows.Err()
}

// attachOpenStatus fills in the open/closed label on a list of shop cards. If the hours cannot be
// loaded the cards simply show no status.
func (h *Handlers) attachOpenStatus(shops []ShopCard) {
	if len(shops) == 0 {
		return
	}
	slugs := make([]string, 0, len(shops))
	for _, s := range shops {
		slugs = append(slugs, s.Slug)
	}
	rows, err := h.DB.Query(`
		SELECT v.slug, d.weekday, d.opens, d.closes
		FROM vendor_hours d JOIN vendors v ON v.id = d.vendor_id
		WHERE v.slug = ANY($1)`, pq.Array(slugs))
	if err != nil {
		return
	}
	defer rows.Close()
	bySlug := map[string]*weekHours{}
	for rows.Next() {
		var slug string
		var wd int
		var d dayHours
		if err := rows.Scan(&slug, &wd, &d.Opens, &d.Closes); err != nil || wd < 0 || wd > 6 {
			continue
		}
		w := bySlug[slug]
		if w == nil {
			w = &weekHours{}
			bySlug[slug] = w
		}
		w[wd] = &d
	}
	now := time.Now()
	for i := range shops {
		if w := bySlug[shops[i].Slug]; w != nil {
			shops[i].OpenKnown, shops[i].OpenNow, shops[i].OpenLabel = openStatus(*w, now)
		}
	}
}

// HoursFormRow is one day of the opening-hours form on the vendor profile page.
type HoursFormRow struct {
	Weekday int
	Name    string
	Open    bool
	Opens   string
	Closes  string
}

func hoursFormRows(w weekHours) []HoursFormRow {
	rows := make([]HoursFormRow, 0, 7)
	for _, wd := range weekdayOrder {
		row := HoursFormRow{Weekday: wd, Name: weekdayNames[wd], Opens: "09:00", Closes: "21:00"}
		if d := w[wd]; d != nil {
			row.Open, row.Opens, row.Closes = true, d.Opens, d.Closes
		}
		rows = append(rows, row)
	}
	return rows
}

type hoursEntry struct {
	Weekday       int
	Opens, Closes string
}

// parseHoursForm reads the opening-hours fields. A ticked day needs a valid opening and closing time.
func parseHoursForm(r *http.Request) ([]hoursEntry, error) {
	var out []hoursEntry
	for _, wd := range weekdayOrder {
		if r.FormValue(fmt.Sprintf("open_%d", wd)) != "on" {
			continue
		}
		o, err1 := time.Parse("15:04", strings.TrimSpace(r.FormValue(fmt.Sprintf("opens_%d", wd))))
		c, err2 := time.Parse("15:04", strings.TrimSpace(r.FormValue(fmt.Sprintf("closes_%d", wd))))
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("choose an opening and closing time for %s", weekdayNames[wd])
		}
		if o.Equal(c) {
			return nil, fmt.Errorf("opening and closing time can't be the same on %s", weekdayNames[wd])
		}
		out = append(out, hoursEntry{wd, o.Format("15:04"), c.Format("15:04")})
	}
	return out, nil
}
