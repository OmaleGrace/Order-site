package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"Order-site/middleware"
)

func TestValidateChatMessage(t *testing.T) {
	cases := []struct {
		name, input, want string
		valid             bool
	}{
		{"normal", " Hello vendor ", "Hello vendor", true},
		{"empty", "  ", "", false},
		{"too long", strings.Repeat("a", 2001), "", false},
		{"at limit", strings.Repeat("a", 2000), strings.Repeat("a", 2000), true},
		{"null byte removed", "hello\x00world", "helloworld", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := validateChatMessage(tc.input)
			if got != tc.want || ok != tc.valid {
				t.Fatalf("validateChatMessage() = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.valid)
			}
		})
	}
}

func TestMessagesShowPicturesAndActiveStatus(t *testing.T) {
	if err := os.Chdir(".."); err == nil {
		defer os.Chdir("handlers")
	}
	db := setupTestDB(t)
	defer db.Close()
	h := New(db)

	var customerID, ownerID, vendorID, threadID int
	db.QueryRow(`INSERT INTO users (name, email, password, profile_picture) VALUES ('Ada Obi', 'ada@example.com', 'x', '/media/7') RETURNING id`).Scan(&customerID)
	db.QueryRow(`INSERT INTO users (name, email, password) VALUES ('Grace Owner', 'grace@example.com', 'x') RETURNING id`).Scan(&ownerID)
	if err := db.QueryRow(`INSERT INTO vendors (name, slug, owner_user_id, logo_url) VALUES ('Grace Kitchen', 'grace-kitchen', $1, '/media/9') RETURNING id`, ownerID).Scan(&vendorID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO message_threads (customer_id, vendor_id) VALUES ($1, $2) RETURNING id`, customerID, vendorID).Scan(&threadID); err != nil {
		t.Fatal(err)
	}
	db.Exec(`INSERT INTO messages (thread_id, sender_user_id, body) VALUES ($1, $2, 'Hello shop')`, threadID, customerID)
	custSession, _ := middleware.CreateSession(db, customerID)
	ownerSession, _ := middleware.CreateSession(db, ownerID)

	get := func(path, session string, handler http.HandlerFunc, thread bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if thread {
			req.SetPathValue("threadID", strconv.Itoa(threadID))
		}
		req.AddCookie(&http.Cookie{Name: "session_token", Value: session})
		w := httptest.NewRecorder()
		handler(w, req)
		return w
	}

	// Nobody has been active yet: pictures show, but no one is marked active.
	inbox := get("/messages", custSession, h.MessagesInbox, false).Body.String()
	if !strings.Contains(inbox, `src="/media/9"`) || strings.Contains(inbox, "av-wrap is-online") {
		t.Fatalf("customer inbox should show the shop logo and nobody active:\n%s", inbox)
	}
	if body := get("/messages", ownerSession, h.MessagesInbox, false).Body.String(); !strings.Contains(body, `src="/media/7"`) {
		t.Fatal("the shop's inbox should show the customer's picture")
	}

	// Opening the inbox marks the viewer as active for others.
	time.Sleep(150 * time.Millisecond)
	var seen sql.NullTime
	db.QueryRow(`SELECT last_seen_at FROM users WHERE id = $1`, customerID).Scan(&seen)
	if !seen.Valid {
		t.Fatal("opening the inbox should record the customer as active")
	}
	db.Exec(`UPDATE users SET last_seen_at = NULL WHERE id = $1`, customerID) // back to "not active" for the checks below

	// The shop owner was active a moment ago.
	db.Exec(`UPDATE users SET last_seen_at = NOW() WHERE id = $1`, ownerID)
	if body := get("/messages", custSession, h.MessagesInbox, false).Body.String(); !strings.Contains(body, "av-wrap is-online") {
		t.Fatal("the customer should see the shop owner as active")
	}
	// ...but the owner is not told the customer is active, because the customer was not.
	if body := get("/messages", ownerSession, h.MessagesInbox, false).Body.String(); strings.Contains(body, "av-wrap is-online") {
		t.Fatal("the shop owner must not see an inactive customer as active")
	}

	thread := get("/messages/"+strconv.Itoa(threadID), custSession, h.MessageThread, true).Body.String()
	if !strings.Contains(thread, "is-online") || !strings.Contains(thread, "Active now") || !strings.Contains(thread, `src="/media/9"`) {
		t.Fatalf("the conversation header should show the shop logo and active status:\n%s", thread)
	}

	// The live poll reports it too, and goes quiet when they are no longer active.
	poll := func() bool {
		req := httptest.NewRequest(http.MethodGet, "/messages/"+strconv.Itoa(threadID)+"/updates?after=0&visible=1", nil)
		req.SetPathValue("threadID", strconv.Itoa(threadID))
		req.AddCookie(&http.Cookie{Name: "session_token", Value: custSession})
		w := httptest.NewRecorder()
		h.MessageUpdates(w, req)
		var out struct {
			OtherOnline bool `json:"other_online"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("bad JSON: %v: %s", err, w.Body.String())
		}
		return out.OtherOnline
	}
	if !poll() {
		t.Fatal("the poll should report the shop owner as active")
	}
	db.Exec(`UPDATE users SET last_seen_at = NOW() - INTERVAL '10 minutes' WHERE id = $1`, ownerID)
	if poll() {
		t.Fatal("someone last seen 10 minutes ago is not active now")
	}
}
