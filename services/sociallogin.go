package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"

	"github.com/pufferpanel/pufferpanel/v3/config"
	"golang.org/x/crypto/hkdf"
)

func EncryptSocialSecret(secret string) (string, error) {
	key, err := socialSecretKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(secret), []byte("social-login-client-secret:v1"))
	return "v1:" + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

func DecryptSocialSecret(encrypted string) (string, error) {
	if len(encrypted) < 4 || encrypted[:3] != "v1:" {
		return "", errors.New("unsupported social login secret format")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(encrypted[3:])
	if err != nil {
		return "", err
	}
	key, err := socialSecretKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("invalid social login secret")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte("social-login-client-secret:v1"))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func socialSecretKey() ([]byte, error) {
	masterKey, err := hex.DecodeString(config.SessionKey.Value())
	if err != nil || len(masterKey) < 32 {
		return nil, errors.New("panel session key is not available")
	}
	key := make([]byte, 32)
	if _, err = io.ReadFull(hkdf.New(sha256.New, masterKey, []byte("pufferpanel"), []byte("social-login client secret encryption v1")), key); err != nil {
		return nil, err
	}
	return key, nil
}
