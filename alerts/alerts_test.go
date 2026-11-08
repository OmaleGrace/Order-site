package alerts

import (
	"Order-site/email"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type capture struct {
	mu   sync.Mutex
	subs []string
	done chan struct{}
}

func (c *capture) send(to, subject, body string) error {
	c.mu.Lock()
	c.subs = append(c.subs, to+"|"+subject+"|"+body)
	c.mu.Unlock()
	select {
	case c.done <- struct{}{}:
	default:
	}
	return nil
}

func setup(t *testing.T) *capture {
	t.Helper()
	mu.Lock()
	lastSent = map[string]time.Time{}
	sentLog = nil
	mu.Unlock()
	t.Setenv("ALERT_EMAIL", "owner@example.com")
	c := &capture{done: make(chan struct{}, 20)}
	send = c.send
	now = time.Now
	t.Cleanup(func() { send = emailSendForTests })
	return c
}

func (c *capture) wait(t *testing.T) {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		t.Fatal("no alert was sent")
	}
}

func TestSameProblemIsMutedForFifteenMinutes(t *testing.T) {
	c := setup(t)
	Notify("k", "subject", "details")
	c.wait(t)
	Notify("k", "subject", "details")
	time.Sleep(50 * time.Millisecond)
	if len(c.subs) != 1 {
		t.Fatalf("want 1 email, got %d", len(c.subs))
	}
	base := time.Now()
	now = func() time.Time { return base.Add(16 * time.Minute) }
	Notify("k", "subject", "details")
	c.wait(t)
	if len(c.subs) != 2 {
		t.Fatalf("want 2 emails after the quiet period, got %d", len(c.subs))
	}
}

func TestAtMostTenAlertsAnHour(t *testing.T) {
	c := setup(t)
	for i := 0; i < 15; i++ {
		Notify("k"+string(rune('a'+i)), "s", "d")
	}
	time.Sleep(200 * time.Millisecond)
	if len(c.subs) != maxPerHour {
		t.Fatalf("want %d emails, got %d", maxPerHour, len(c.subs))
	}
}

func TestNoRecipientMeansNoEmail(t *testing.T) {
	c := setup(t)
	t.Setenv("ALERT_EMAIL", "")
	t.Setenv("MAIN_ADMIN_EMAIL", "")
	Notify("k", "s", "d")
	time.Sleep(50 * time.Millisecond)
	if len(c.subs) != 0 {
		t.Fatal("an alert was sent although no address is set")
	}
}

func TestFallsBackToMainAdminEmail(t *testing.T) {
	setup(t)
	t.Setenv("ALERT_EMAIL", "")
	t.Setenv("MAIN_ADMIN_EMAIL", "admin@example.com")
	if Recipient() != "admin@example.com" {
		t.Fatal("did not fall back to MAIN_ADMIN_EMAIL")
	}
}

func TestPanicGivesAPlain500AndAnAlertWithoutTheQueryString(t *testing.T) {
	c := setup(t)
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/orders?token=SECRET", nil))
	if w.Code != 500 {
		t.Fatalf("status %d", w.Code)
	}
	c.wait(t)
	if strings.Contains(c.subs[0], "SECRET") {
		t.Fatal("the query string leaked into the alert")
	}
	if !strings.Contains(c.subs[0], "/orders") || !strings.Contains(c.subs[0], "boom") {
		t.Fatalf("alert is missing the path or the message: %s", c.subs[0])
	}
}

func TestServerErrorRaisesAnAlertButHealthyPagesAndHealthzDoNot(t *testing.T) {
	c := setup(t)
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad", "/healthz":
			http.Error(w, "x", 503)
		default:
			w.Write([]byte("ok"))
		}
	}))
	for _, p := range []string{"/fine", "/healthz", "/missing-ok"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", p, nil))
	}
	time.Sleep(50 * time.Millisecond)
	if len(c.subs) != 0 {
		t.Fatalf("unexpected alert: %v", c.subs)
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/bad", nil))
	c.wait(t)
}

var emailSendForTests = email.Send
