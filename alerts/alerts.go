// Package alerts emails the site owner when something breaks, so problems are noticed before
// customers report them.
//
// Settings:
//
//	ALERT_EMAIL   where alerts go (falls back to MAIN_ADMIN_EMAIL). Leave both empty to switch
//	              alerts off; the site then behaves exactly as before.
//
// To protect the inbox (and the email quota) the same problem is mailed at most once every
// 15 minutes, and no more than 10 alert emails go out in any hour.
package alerts

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"Order-site/brand"
	"Order-site/email"
	"Order-site/safe"
)

const (
	sameProblemEvery = 15 * time.Minute
	maxPerHour       = 10
)

var (
	mu       sync.Mutex
	lastSent = map[string]time.Time{}
	sentLog  []time.Time

	// send is replaced in tests.
	send = email.Send
	now  = time.Now
)

// Recipient returns the alert address, or "" when alerts are switched off.
func Recipient() string {
	for _, k := range []string{"ALERT_EMAIL", "MAIN_ADMIN_EMAIL"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// allow applies the throttle. key identifies the problem.
func allow(key string) bool {
	mu.Lock()
	defer mu.Unlock()
	t := now()
	if last, ok := lastSent[key]; ok && t.Sub(last) < sameProblemEvery {
		return false
	}
	recent := sentLog[:0]
	for _, s := range sentLog {
		if t.Sub(s) < time.Hour {
			recent = append(recent, s)
		}
	}
	sentLog = recent
	if len(sentLog) >= maxPerHour {
		return false
	}
	lastSent[key] = t
	sentLog = append(sentLog, t)
	if len(lastSent) > 500 { // keep the map small on a long-running server
		for k, v := range lastSent {
			if t.Sub(v) > sameProblemEvery {
				delete(lastSent, k)
			}
		}
	}
	return true
}

// Notify emails an alert in the background. key groups repeats of the same problem.
func Notify(key, subject, details string) {
	to := Recipient()
	if to == "" || !allow(key) {
		return
	}
	go func() {
		defer safe.Recover("send alert email")
		body := fmt.Sprintf(`<div style="font-family:system-ui,sans-serif;max-width:640px"><h2 style="margin:0 0 8px">%s</h2><p style="color:#555;margin:0 0 12px">%s</p><pre style="white-space:pre-wrap;background:#f5f5f5;padding:12px;border-radius:8px;font-size:13px">%s</pre><p style="color:#888;font-size:12px">Repeats of this problem are muted for 15 minutes. Set ALERT_EMAIL to change where these go.</p></div>`,
			html.EscapeString(subject), html.EscapeString(now().UTC().Format("2 Jan 2006 15:04:05 MST")), html.EscapeString(details))
		if err := send(to, "["+brand.Name()+" alert] "+subject, body); err != nil {
			log.Printf("alerts: could not send alert email: %v", err)
		}
	}()
}

// Install makes panics in background jobs raise an alert too.
func Install() {
	safe.OnPanic = func(job string, v any, stack []byte) {
		Notify("job:"+job, "Background job crashed: "+job, fmt.Sprintf("%v\n\n%s", v, trim(string(stack), 3000)))
	}
}

func trim(s string, n int) string {
	if len(s) > n {
		return s[:n] + "\n…"
	}
	return s
}

// statusWriter remembers the status code a handler sent.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status, w.wrote = code, true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.status, w.wrote = http.StatusOK, true
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController (and Flusher/Hijacker checks) see the real writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Middleware wraps the whole site. A request that panics gets a plain 500 page (instead of a
// dropped connection) and an alert; any response with a 5xx status raises an alert as well.
// Only the method and path are reported: no query string, cookies, form data or addresses.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w}
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				log.Printf("PANIC serving %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				if !sw.wrote {
					http.Error(sw, "Something went wrong on our side. Please try again in a moment.", http.StatusInternalServerError)
				}
				Notify("panic:"+r.Method+" "+r.URL.Path, "Page crashed: "+r.Method+" "+r.URL.Path,
					fmt.Sprintf("%v\n\n%s", rec, trim(string(debug.Stack()), 3000)))
				return
			}
			if sw.status >= 500 && r.URL.Path != "/healthz" {
				Notify(fmt.Sprintf("5xx:%s %s %d", r.Method, r.URL.Path, sw.status),
					fmt.Sprintf("Error %d on %s %s", sw.status, r.Method, r.URL.Path),
					"The server answered with an error. Check the host's logs around this time for the cause.")
			}
		}()
		next.ServeHTTP(sw, r)
	})
}
