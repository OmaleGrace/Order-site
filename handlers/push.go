package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"Order-site/push"
	"Order-site/safe"
)

func writePushJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// PushKey gives the browser the public key it needs to subscribe. It is public by design.
func (h *Handlers) PushKey(w http.ResponseWriter, r *http.Request) {
	writePushJSON(w, http.StatusOK, map[string]any{"enabled": push.Enabled(), "key": push.PublicKey()})
}

// PushStatus tells the signed-in person how many of their devices receive notifications.
func (h *Handlers) PushStatus(w http.ResponseWriter, r *http.Request) {
	userID, err := requestUserID(h, r)
	if err != nil {
		writePushJSON(w, http.StatusUnauthorized, map[string]string{"error": "Please log in."})
		return
	}
	writePushJSON(w, http.StatusOK, map[string]any{"enabled": push.Enabled(), "key": push.PublicKey(), "devices": push.Count(h.DB, userID)})
}

type pushSubscribeBody struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// PushSubscribe saves this device so it can receive notifications.
func (h *Handlers) PushSubscribe(w http.ResponseWriter, r *http.Request) {
	userID, err := requestUserID(h, r)
	if err != nil {
		writePushJSON(w, http.StatusUnauthorized, map[string]string{"error": "Please log in."})
		return
	}
	if !push.Enabled() {
		writePushJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Notifications are not set up on this site yet."})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var body pushSubscribeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !push.ValidEndpoint(body.Endpoint) || body.Keys.P256dh == "" || body.Keys.Auth == "" || len(body.Keys.P256dh) > 200 || len(body.Keys.Auth) > 60 {
		writePushJSON(w, http.StatusBadRequest, map[string]string{"error": "That notification subscription is not valid."})
		return
	}
	if err := push.Save(h.DB, userID, push.Subscription{Endpoint: body.Endpoint, P256dh: body.Keys.P256dh, Auth: body.Keys.Auth}, r.UserAgent()); err != nil {
		writePushJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not save your notification setting."})
		return
	}
	writePushJSON(w, http.StatusOK, map[string]any{"ok": true, "devices": push.Count(h.DB, userID)})
}

// PushUnsubscribe turns notifications off for this device.
func (h *Handlers) PushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	userID, err := requestUserID(h, r)
	if err != nil {
		writePushJSON(w, http.StatusUnauthorized, map[string]string{"error": "Please log in."})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Endpoint) == "" {
		writePushJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing device."})
		return
	}
	_ = push.Remove(h.DB, userID, body.Endpoint)
	writePushJSON(w, http.StatusOK, map[string]any{"ok": true, "devices": push.Count(h.DB, userID)})
}

// PushTest sends a real notification to all of the person's devices and reports what happened,
// so they can see right away that it works.
func (h *Handlers) PushTest(w http.ResponseWriter, r *http.Request) {
	userID, err := requestUserID(h, r)
	if err != nil {
		writePushJSON(w, http.StatusUnauthorized, map[string]string{"error": "Please log in."})
		return
	}
	if !push.Enabled() {
		writePushJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Notifications are not set up on this site yet."})
		return
	}
	res := push.NotifyUser(h.DB, userID, push.Payload{
		Title: "Notifications are on",
		Body:  "You'll get order updates and messages here, even when the app is closed.",
		URL:   "/account",
		Tag:   "push-test",
	})
	writePushJSON(w, http.StatusOK, map[string]any{"devices": res.Devices, "sent": res.Sent, "removed": res.Removed, "failed": res.Failed})
}

// pushToUser sends a notification in the background. Failures are logged, never shown.
func pushToUser(h *Handlers, userID int, p push.Payload) {
	if !push.Enabled() {
		return
	}
	go func() {
		defer safe.Recover("push notification")
		push.NotifyUser(h.DB, userID, p)
	}()
}
