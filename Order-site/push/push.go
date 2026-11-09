// Package push sends Web Push notifications (the kind that appear on a phone's lock screen even
// when the app is closed) using only the Go standard library:
//
//	RFC 8291  message encryption (aes128gcm)
//	RFC 8292  VAPID, which proves to the phone's push service that the message is from this site
//
// Settings (run `go run ./cmd/vapid-keys` once to create the first two):
//
//	VAPID_PUBLIC_KEY   shared with browsers
//	VAPID_PRIVATE_KEY  secret: keep it only in the host's environment settings
//	VAPID_SUBJECT      contact address such as mailto:you@example.com (falls back to ALERT_EMAIL)
//
// Without the keys push is simply switched off and the rest of the site is unaffected.
package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var b64 = base64.RawURLEncoding

// Payload is what the service worker receives and shows.
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`           // page to open when the notification is tapped
	Tag   string `json:"tag,omitempty"` // a newer notification with the same tag replaces the older
}

// Subscription is one phone/browser that agreed to receive notifications.
type Subscription struct {
	Endpoint string
	P256dh   string // the browser's public key, base64url
	Auth     string // the browser's auth secret, base64url
}

type keys struct {
	pub     string
	priv    *ecdsa.PrivateKey
	subject string
}

func loadKeys() (*keys, error) {
	pub := strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY"))
	priv := strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY"))
	if pub == "" || priv == "" {
		return nil, errors.New("VAPID keys are not set")
	}
	k, err := parseKeys(pub, priv)
	if err != nil {
		return nil, err
	}
	k.subject = strings.TrimSpace(os.Getenv("VAPID_SUBJECT"))
	if k.subject == "" {
		if e := strings.TrimSpace(os.Getenv("ALERT_EMAIL")); e != "" {
			k.subject = "mailto:" + e
		} else if e := strings.TrimSpace(os.Getenv("MAIN_ADMIN_EMAIL")); e != "" {
			k.subject = "mailto:" + e
		} else {
			k.subject = "https://localhost"
		}
	}
	if !strings.HasPrefix(k.subject, "mailto:") && !strings.HasPrefix(k.subject, "https://") {
		k.subject = "mailto:" + k.subject
	}
	return k, nil
}

func parseKeys(pub, priv string) (*keys, error) {
	pb, err := b64.DecodeString(pub)
	if err != nil || len(pb) != 65 || pb[0] != 4 {
		return nil, errors.New("VAPID_PUBLIC_KEY must be a 65-byte uncompressed P-256 key in base64url")
	}
	db, err := b64.DecodeString(priv)
	if err != nil || len(db) != 32 {
		return nil, errors.New("VAPID_PRIVATE_KEY must be 32 bytes in base64url")
	}
	d := new(big.Int).SetBytes(db)
	key := &ecdsa.PrivateKey{D: d}
	key.PublicKey.Curve = elliptic.P256()
	key.PublicKey.X, key.PublicKey.Y = elliptic.P256().ScalarBaseMult(db)
	want := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)
	if !bytes.Equal(want, pb) {
		return nil, errors.New("VAPID_PUBLIC_KEY does not belong to VAPID_PRIVATE_KEY")
	}
	return &keys{pub: pub, priv: key}, nil
}

// GenerateKeys makes a new VAPID key pair (both base64url).
func GenerateKeys() (public, private string, err error) {
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return b64.EncodeToString(k.PublicKey().Bytes()), b64.EncodeToString(k.Bytes()), nil
}

var current *keys

// Init reads the VAPID settings. It returns an error when push is not configured; the caller
// can log it and carry on without push.
func Init() error {
	k, err := loadKeys()
	if err != nil {
		current = nil
		return err
	}
	current = k
	return nil
}

// Enabled reports whether notifications can be sent.
func Enabled() bool { return current != nil }

// PublicKey is what the browser needs to subscribe.
func PublicKey() string {
	if current == nil {
		return ""
	}
	return current.pub
}

// Endpoints must belong to a real browser push service. Without this check a person could
// "subscribe" with any address and make the server send requests to it.
var allowedHostSuffixes = []string{
	"fcm.googleapis.com",
	"push.services.mozilla.com",
	"push.apple.com",
	"notify.windows.com",
	"push.microsoft.com",
}

// ValidEndpoint reports whether endpoint is an https address on a known push service.
func ValidEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || len(endpoint) > 2000 {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, s := range allowedHostSuffixes {
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}

// encrypt implements RFC 8291. senderPriv and salt are normally random; tests pass fixed ones.
func encrypt(plaintext []byte, p256dh, auth string, senderPriv *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	uaPubBytes, err := b64.DecodeString(strings.TrimRight(p256dh, "="))
	if err != nil {
		return nil, fmt.Errorf("bad p256dh: %w", err)
	}
	authSecret, err := b64.DecodeString(strings.TrimRight(auth, "="))
	if err != nil || len(authSecret) != 16 {
		return nil, errors.New("bad auth secret")
	}
	uaPub, err := ecdh.P256().NewPublicKey(uaPubBytes)
	if err != nil {
		return nil, fmt.Errorf("bad p256dh key: %w", err)
	}
	if senderPriv == nil {
		if senderPriv, err = ecdh.P256().GenerateKey(rand.Reader); err != nil {
			return nil, err
		}
	}
	if salt == nil {
		salt = make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return nil, err
		}
	}
	shared, err := senderPriv.ECDH(uaPub)
	if err != nil {
		return nil, err
	}
	asPub := senderPriv.PublicKey().Bytes()

	keyInfo := append(append([]byte("WebPush: info\x00"), uaPubBytes...), asPub...)
	ikm, err := hkdf.Key(sha256.New, shared, authSecret, string(keyInfo), 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// One record: plaintext, then the 0x02 "last record" delimiter.
	sealed := gcm.Seal(nil, nonce, append(append([]byte{}, plaintext...), 0x02), nil)

	var out bytes.Buffer
	out.Write(salt)
	var rs [4]byte
	binary.BigEndian.PutUint32(rs[:], 4096)
	out.Write(rs[:])
	out.WriteByte(byte(len(asPub)))
	out.Write(asPub)
	out.Write(sealed)
	return out.Bytes(), nil
}

// vapidHeader builds the Authorization header for the push service that owns endpoint.
func (k *keys) vapidHeader(endpoint string, now time.Time) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	header := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, _ := json.Marshal(map[string]any{
		"aud": u.Scheme + "://" + u.Host,
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": k.subject,
	})
	signing := header + "." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, k.priv, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return "vapid t=" + signing + "." + b64.EncodeToString(sig) + ", k=" + k.pub, nil
}

// ErrGone means the phone's push service says this subscription no longer exists (the app was
// uninstalled or notifications were switched off). The caller should delete it.
var ErrGone = errors.New("push subscription is gone")

var client = &http.Client{Timeout: 15 * time.Second}

// Send delivers one notification. It returns ErrGone when the subscription should be removed.
func Send(ctx context.Context, sub Subscription, p Payload) error {
	if current == nil {
		return errors.New("push is not configured")
	}
	if !ValidEndpoint(sub.Endpoint) {
		return ErrGone
	}
	p.Body = truncate(p.Body, 300)
	p.Title = truncate(p.Title, 80)
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	body, err := encrypt(raw, sub.P256dh, sub.Auth, nil, nil)
	if err != nil {
		return ErrGone // an unusable key pair can never succeed
	}
	auth, err := current.vapidHeader(sub.Endpoint, time.Now())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", "86400")
	req.Header.Set("Urgency", "high")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	default:
		return fmt.Errorf("push service answered %d", resp.StatusCode)
	}
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
