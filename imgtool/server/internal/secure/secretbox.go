package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

const secretBoxPrefix = "v1:"

type SecretBox struct {
	key [32]byte
}

func NewSecretBox(secret string) SecretBox {
	return SecretBox{key: sha256.Sum256([]byte(secret))}
}

func (b SecretBox) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(b.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return secretBoxPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (b SecretBox) Decrypt(encoded string) (string, error) {
	if !strings.HasPrefix(encoded, secretBoxPrefix) {
		return "", errors.New("unsupported encrypted value")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, secretBoxPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(b.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) <= gcm.NonceSize() {
		return "", errors.New("encrypted value is too short")
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
