package push

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func dec(t *testing.T, s string) []byte {
	t.Helper()
	b, err := b64.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The worked example from RFC 8291 Appendix A. If our encryption differs by even one byte, the
// phone cannot decrypt the message, so matching this vector is the proof the format is right.
func TestEncryptMatchesRFC8291AppendixA(t *testing.T) {
	asPriv, err := ecdh.P256().NewPrivateKey(dec(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := encrypt([]byte("When I grow up, I want to be a watermelon"),
		"BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
		"BTBZMqHH6r4Tts7J_aSIgg", asPriv, dec(t, "DGv6ra1nlYgDCS1FRnbzlw"))
	if err != nil {
		t.Fatal(err)
	}
	want := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if b64.EncodeToString(got) != want {
		t.Fatalf("encrypted body differs from the RFC example:\n got %s\nwant %s", b64.EncodeToString(got), want)
	}
}

func TestVapidHeaderIsAValidSignedToken(t *testing.T) {
	pub, priv, err := GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	k, err := parseKeys(pub, priv)
	if err != nil {
		t.Fatal(err)
	}
	k.subject = "mailto:owner@example.com"
	h, err := k.vapidHeader("https://fcm.googleapis.com/fcm/send/abc", time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(h, ", k="+pub) || !strings.HasPrefix(h, "vapid t=") {
		t.Fatalf("bad header %q", h)
	}
	jwt := strings.TrimSuffix(strings.TrimPrefix(h, "vapid t="), ", k="+pub)
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts", len(parts))
	}
	var claims map[string]any
	if err := json.Unmarshal(dec(t, parts[1]), &claims); err != nil {
		t.Fatal(err)
	}
	if claims["aud"] != "https://fcm.googleapis.com" || claims["sub"] != "mailto:owner@example.com" {
		t.Fatalf("claims %v", claims)
	}
	sig := dec(t, parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if len(sig) != 64 || !ecdsa.Verify(&k.priv.PublicKey, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("signature does not verify")
	}
}

func TestKeysMustMatch(t *testing.T) {
	pub, _, _ := GenerateKeys()
	_, priv2, _ := GenerateKeys()
	if _, err := parseKeys(pub, priv2); err == nil {
		t.Fatal("mismatched keys were accepted")
	}
}

func TestOnlyRealPushServicesAreAllowed(t *testing.T) {
	ok := []string{"https://fcm.googleapis.com/fcm/send/x", "https://updates.push.services.mozilla.com/wpush/v2/x", "https://web.push.apple.com/x", "https://wns2-par02p.notify.windows.com/x"}
	bad := []string{"http://fcm.googleapis.com/x", "https://evil.example.com/x", "https://fcm.googleapis.com.evil.com/x", "https://localhost/x", "https://127.0.0.1/x", "https://user@fcm.googleapis.com/x", "ftp://fcm.googleapis.com/x", ""}
	for _, u := range ok {
		if !ValidEndpoint(u) {
			t.Errorf("rejected %s", u)
		}
	}
	for _, u := range bad {
		if ValidEndpoint(u) {
			t.Errorf("accepted %s", u)
		}
	}
}

// End to end: a fake push service receives the request, and we decrypt it the way a phone would.
func TestSendReachesThePushServiceAndDecrypts(t *testing.T) {
	pub, priv, _ := GenerateKeys()
	t.Setenv("VAPID_PUBLIC_KEY", pub)
	t.Setenv("VAPID_PRIVATE_KEY", priv)
	t.Setenv("VAPID_SUBJECT", "mailto:o@example.com")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	uaPriv, _ := ecdh.P256().GenerateKey(rngReader{})
	auth := b64.EncodeToString([]byte("0123456789abcdef"))
	var gotBody []byte
	var gotHdr http.Header
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHdr = r.Header.Clone()
		buf := make([]byte, 8192)
		n, _ := r.Body.Read(buf)
		gotBody = buf[:n]
		w.WriteHeader(201)
	}))
	defer srv.Close()
	oldClient, oldAllowed := client, allowedHostSuffixes
	client = srv.Client()
	allowedHostSuffixes = append(allowedHostSuffixes, "127.0.0.1")
	defer func() { client, allowedHostSuffixes = oldClient, oldAllowed }()

	err := Send(context.Background(), Subscription{Endpoint: srv.URL + "/push/1", P256dh: b64.EncodeToString(uaPriv.PublicKey().Bytes()), Auth: auth},
		Payload{Title: "Order ready", Body: "Come and get it", URL: "/orders"})
	if err != nil {
		t.Fatal(err)
	}
	if gotHdr.Get("Content-Encoding") != "aes128gcm" || !strings.HasPrefix(gotHdr.Get("Authorization"), "vapid t=") || gotHdr.Get("TTL") == "" {
		t.Fatalf("headers: %v", gotHdr)
	}
	plain := decryptAsPhone(t, gotBody, uaPriv, []byte("0123456789abcdef"))
	var p Payload
	if err := json.Unmarshal(plain, &p); err != nil || p.Title != "Order ready" || p.URL != "/orders" {
		t.Fatalf("phone decrypted %q (%v)", plain, err)
	}
}

func TestGoneSubscriptionsAreReported(t *testing.T) {
	pub, priv, _ := GenerateKeys()
	t.Setenv("VAPID_PUBLIC_KEY", pub)
	t.Setenv("VAPID_PRIVATE_KEY", priv)
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	uaPriv, _ := ecdh.P256().GenerateKey(rngReader{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(410) }))
	defer srv.Close()
	oldClient, oldAllowed := client, allowedHostSuffixes
	client = srv.Client()
	allowedHostSuffixes = append(allowedHostSuffixes, "127.0.0.1")
	defer func() { client, allowedHostSuffixes = oldClient, oldAllowed }()
	err := Send(context.Background(), Subscription{Endpoint: srv.URL + "/x", P256dh: b64.EncodeToString(uaPriv.PublicKey().Bytes()), Auth: b64.EncodeToString(make([]byte, 16))}, Payload{Title: "t"})
	if err != ErrGone {
		t.Fatalf("want ErrGone, got %v", err)
	}
}
