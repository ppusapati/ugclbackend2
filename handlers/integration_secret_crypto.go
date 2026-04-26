package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const integrationSecretKeyEnv = "THIRD_PARTY_INTEGRATION_ENCRYPTION_KEY"

func encryptIntegrationSecret(plain string) (string, error) {
	key, err := getIntegrationEncryptionKey()
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
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)
	payload := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func decryptIntegrationSecret(cipherText string) (string, error) {
	key, err := getIntegrationEncryptionKey()
	if err != nil {
		return "", err
	}

	payload, err := base64.StdEncoding.DecodeString(cipherText)
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

	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return "", errors.New("invalid encrypted integration secret payload")
	}

	nonce, ciphertext := payload[:nonceSize], payload[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}

func getIntegrationEncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(integrationSecretKeyEnv))
	if raw == "" {
		return nil, fmt.Errorf("%s is required", integrationSecretKeyEnv)
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil && (len(decoded) == 16 || len(decoded) == 24 || len(decoded) == 32) {
		return decoded, nil
	}

	if len(raw) == 16 || len(raw) == 24 || len(raw) == 32 {
		return []byte(raw), nil
	}

	return nil, fmt.Errorf("%s must be base64-encoded AES key or a raw 16/24/32-byte value", integrationSecretKeyEnv)
}
