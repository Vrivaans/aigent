package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	envDBEncryptionKey = "DB_ENCRYPTION_KEY"
	envJWTSecret       = "JWT_SECRET"
	minJWTSecretLen    = 16
	dbEncryptionKeyLen = 32
)

// JWTSecretKey returns the HS256 signing key for session JWTs.
func JWTSecretKey() []byte {
	return []byte(strings.TrimSpace(os.Getenv(envJWTSecret)))
}

// ValidateDBEncryptionKey ensures the AES-256-GCM master key length is correct.
func ValidateDBEncryptionKey(key string) error {
	if len(key) != dbEncryptionKeyLen {
		return fmt.Errorf("%s must be exactly %d characters long (for AES-256), got %d", envDBEncryptionKey, dbEncryptionKeyLen, len(key))
	}
	return nil
}

// ValidateJWTSecret ensures a dedicated JWT signing secret is configured.
func ValidateJWTSecret(secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return fmt.Errorf("%s is required", envJWTSecret)
	}
	if len(secret) < minJWTSecretLen {
		return fmt.Errorf("%s must be at least %d characters", envJWTSecret, minJWTSecretLen)
	}
	return nil
}

// ValidateStartupSecrets checks both secrets before the server accepts traffic.
func ValidateStartupSecrets() error {
	if err := ValidateDBEncryptionKey(os.Getenv(envDBEncryptionKey)); err != nil {
		return err
	}
	if err := ValidateJWTSecret(os.Getenv(envJWTSecret)); err != nil {
		return err
	}
	return nil
}

// ErrMissingSecret is returned when a required secret env var is absent.
var ErrMissingSecret = errors.New("missing required secret")
