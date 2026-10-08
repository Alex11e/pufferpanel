package services

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/pufferpanel/pufferpanel/v3/config"
)

func setSocialTestSessionKey(t *testing.T) {
	t.Helper()
	previous := config.SessionKey.Value()
	if err := config.SessionKey.Set(strings.Repeat("11", 32), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.SessionKey.Set(previous, false) })
}

func TestSocialSecretEncryptionRoundTrip(t *testing.T) {
	setSocialTestSessionKey(t)
	secret := "provider-secret-value"
	encrypted, err := EncryptSocialSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encrypted, secret) {
		t.Fatal("encrypted secret contains plaintext")
	}
	decrypted, err := DecryptSocialSecret(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != secret {
		t.Fatalf("decrypted secret = %q, want %q", decrypted, secret)
	}
}

func TestSocialSecretEncryptionRejectsTampering(t *testing.T) {
	setSocialTestSessionKey(t)
	encrypted, err := EncryptSocialSecret("provider-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(encrypted, "v1:"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	tampered := "v1:" + base64.RawStdEncoding.EncodeToString(ciphertext)
	if _, err = DecryptSocialSecret(tampered); err == nil {
		t.Fatal("tampered secret was accepted")
	}
}
