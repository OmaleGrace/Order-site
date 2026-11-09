package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"Order-site/middleware"
	"Order-site/push"
)

func pushSetup(t *testing.T) (*Handlers, int, *http.Cookie) {
	t.Helper()
	if err := os.Chdir(".."); err == nil {
		t.Cleanup(func() { os.Chdir("handlers") })
	}
	db := setupTestDB(t)
	t.Cleanup(func() { db.Close() })
	pub, priv, _ := push.GenerateKeys()
	t.Setenv("VAPID_PUBLIC_KEY", pub)
	t.Setenv("VAPID_PRIVATE_KEY", priv)
	if err := push.Init(); err != nil {
		t.Fatal(err)
	}
	var uid int
	db.QueryRow(`INSERT INTO users (name, email, password) VALUES ('P', 'p@example.com', 'x') RETURNING id`).Scan(&uid)
	tok, _ := middleware.CreateSession(db, uid)
	return New(db), uid, &http.Cookie{Name: "session_token", Value: tok}
}

func TestPushSubscribeRejectsUnknownHostsAndAcceptsRealOnes(t *testing.T) {
	h, uid, ck := pushSetup(t)
	post := func(body string) int {
		r := httptest.NewRequest(http.MethodPost, "/push/subscribe", strings.NewReader(body))
		r.AddCookie(ck)
		w := httptest.NewRecorder()
		h.PushSubscribe(w, r)
		return w.Code
	}
	if c := post(`{"endpoint":"https://evil.example.com/x","keys":{"p256dh":"a","auth":"b"}}`); c != 400 {
		t.Fatalf("an unknown host was accepted (%d)", c)
	}
	if c := post(`{"endpoint":"https://fcm.googleapis.com/fcm/send/abc","keys":{"p256dh":"BCVxsr7N","auth":"BTBZMqHH"}}`); c != 200 {
		t.Fatalf("a real push endpoint was refused (%d)", c)
	}
	if push.Count(h.DB, uid) != 1 {
		t.Fatal("the subscription was not stored")
	}
	// Subscribing the same device again does not make a second row.
	post(`{"endpoint":"https://fcm.googleapis.com/fcm/send/abc","keys":{"p256dh":"BCVxsr7N","auth":"BTBZMqHH"}}`)
	if push.Count(h.DB, uid) != 1 {
		t.Fatal("duplicate row")
	}
}

func TestPushNeedsLoginAndUnsubscribeOnlyTouchesMyDevices(t *testing.T) {
	h, uid, ck := pushSetup(t)
	w := httptest.NewRecorder()
	h.PushTest(w, httptest.NewRequest(http.MethodPost, "/push/test", nil))
	if w.Code != 401 {
		t.Fatalf("logged-out test returned %d", w.Code)
	}
	var other int
	h.DB.QueryRow(`INSERT INTO users (name, email, password) VALUES ('Q', 'q@example.com', 'x') RETURNING id`).Scan(&other)
	push.Save(h.DB, other, push.Subscription{Endpoint: "https://fcm.googleapis.com/fcm/send/theirs", P256dh: "k", Auth: "a"}, "")
	push.Save(h.DB, uid, push.Subscription{Endpoint: "https://fcm.googleapis.com/fcm/send/mine", P256dh: "k", Auth: "a"}, "")
	r := httptest.NewRequest(http.MethodPost, "/push/unsubscribe", strings.NewReader(`{"endpoint":"https://fcm.googleapis.com/fcm/send/theirs"}`))
	r.AddCookie(ck)
	h.PushUnsubscribe(httptest.NewRecorder(), r)
	if push.Count(h.DB, other) != 1 {
		t.Fatal("I was able to remove someone else's device")
	}
	r = httptest.NewRequest(http.MethodPost, "/push/unsubscribe", strings.NewReader(`{"endpoint":"https://fcm.googleapis.com/fcm/send/mine"}`))
	r.AddCookie(ck)
	h.PushUnsubscribe(httptest.NewRecorder(), r)
	if push.Count(h.DB, uid) != 0 {
		t.Fatal("my device was not removed")
	}
}
