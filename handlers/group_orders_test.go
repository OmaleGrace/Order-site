package handlers

import (
	"database/sql"
	"testing"
)

func TestGroupState(t *testing.T) {
	cases := []struct {
		status      string
		orderID     sql.NullInt64
		orderStatus sql.NullString
		wantState   string
		wantOrder   string
	}{
		{"requested", sql.NullInt64{}, sql.NullString{}, "requested", ""},
		{"quoted", sql.NullInt64{}, sql.NullString{}, "quoted", ""},
		{"accepted", sql.NullInt64{Int64: 4, Valid: true}, sql.NullString{String: "pending", Valid: true}, "awaiting_payment", ""},
		{"accepted", sql.NullInt64{Int64: 4, Valid: true}, sql.NullString{String: "scheduled", Valid: true}, "paid", "scheduled"},
		{"declined", sql.NullInt64{}, sql.NullString{}, "declined", ""},
	}
	for _, c := range cases {
		state, order := groupState(c.status, c.orderID, c.orderStatus)
		if state != c.wantState || order != c.wantOrder {
			t.Errorf("groupState(%q) = %q, %q; want %q, %q", c.status, state, order, c.wantState, c.wantOrder)
		}
	}
}
