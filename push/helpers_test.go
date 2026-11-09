package push

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"testing"
)

type rngReader struct{}

func (rngReader) Read(p []byte) (int, error) { return rand.Read(p) }

// decryptAsPhone is what a browser does with an incoming push message (RFC 8291 section 3.4).
func decryptAsPhone(t *testing.T, body []byte, uaPriv *ecdh.PrivateKey, authSecret []byte) []byte {
	t.Helper()
	salt := body[:16]
	idLen := int(body[20])
	asPubBytes := body[21 : 21+idLen]
	ct := body[21+idLen:]
	asPub, err := ecdh.P256().NewPublicKey(asPubBytes)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := uaPriv.ECDH(asPub)
	if err != nil {
		t.Fatal(err)
	}
	keyInfo := append(append([]byte("WebPush: info\x00"), uaPriv.PublicKey().Bytes()...), asPubBytes...)
	ikm, _ := hkdf.Key(sha256.New, shared, authSecret, string(keyInfo), 32)
	cek, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		t.Fatalf("phone could not decrypt: %v", err)
	}
	if plain[len(plain)-1] != 0x02 {
		t.Fatal("missing record delimiter")
	}
	return plain[:len(plain)-1]
}
