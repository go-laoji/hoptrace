package data

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const encPrefix = "enc:"

// Cipher 字段级 AES-256-GCM 加解密。
type Cipher struct {
	aead cipher.AEAD
}

func NewCipher(secret string) (*Cipher, error) {
	if secret == "" {
		secret = "hoptrace-dev-secret-change-me"
	}
	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if strings.HasPrefix(plain, encPrefix) {
		return plain, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func (c *Cipher) Decrypt(raw string) (string, error) {
	if raw == "" || !strings.HasPrefix(raw, encPrefix) {
		return raw, nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, encPrefix))
	if err != nil {
		return "", err
	}
	ns := c.aead.NonceSize()
	if len(data) < ns {
		return "", errors.New("ciphertext too short")
	}
	plain, err := c.aead.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
