package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Order-site/testutil"
)

// Calls the real handler (not a copy of its template functions), so a template that uses
// a function the handler forgot to register fails here instead of showing visitors a 500.
func TestAdminDashboardRenders(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	New(db).AdminDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); !strings.Contains(body, "Today's sales") || !strings.Contains(body, "₦0") {
		t.Fatalf("dashboard did not render its sales figures:\n%s", body)
	}
}
