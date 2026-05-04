package handlers

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
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

// EnsureIntegrationEncryptionKey is called once at startup.
// If THIRD_PARTY_INTEGRATION_ENCRYPTION_KEY is not set it generates a
// cryptographically-random AES-256 key, writes it into the .env file so it
// survives restarts, and sets it in the current process environment so it is
// immediately usable without a restart.
var ensureIntegrationKeyOnce sync.Once

func EnsureIntegrationEncryptionKey() {
	ensureIntegrationKeyOnce.Do(func() {
		if strings.TrimSpace(os.Getenv(integrationSecretKeyEnv)) != "" {
			return // already set — nothing to do
		}

		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: could not auto-generate %s: %v\n", integrationSecretKeyEnv, err)
			return
		}
		key := base64.StdEncoding.EncodeToString(b)

		// Persist into .env so the key survives process restarts.
		if err := appendToEnvFile(integrationSecretKeyEnv, key); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: generated %s but could not write to .env: %v — set it manually\n", integrationSecretKeyEnv, err)
		}

		os.Setenv(integrationSecretKeyEnv, key) //nolint:errcheck
		fmt.Printf("INFO: auto-generated %s and persisted to .env\n", integrationSecretKeyEnv)
	})
}

// appendToEnvFile adds KEY=VALUE at the end of .env if the key is not already
// present as an uncommented assignment.
func appendToEnvFile(key, value string) error {
	const envFile = ".env"
	f, err := os.OpenFile(envFile, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, key+"=") {
			return nil // already present, leave as-is
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "\n# Auto-generated AES-256 key for encrypting integration secrets at rest\n%s=%s\n", key, value)
	return err
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
