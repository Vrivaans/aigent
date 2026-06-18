package secrets

import (
	"fmt"
	"os"
	"strings"
)

const (
	KeyDBEncryption = "DB_ENCRYPTION_KEY"
	KeyJWTSecret    = "JWT_SECRET"
)

// Provider resolves named secrets. Default: environment variables (Docker/K8s secretKeyRef).
type Provider interface {
	GetSecret(key string) string
}

// VaultProvider is a placeholder for external secret stores (HashiCorp Vault, AWS SM, etc.).
type VaultProvider interface {
	Provider
	Ready() bool
}

// EnvProvider reads secrets from process environment (default).
type EnvProvider struct{}

func (EnvProvider) GetSecret(key string) string {
	return os.Getenv(key)
}

// ProviderFunc adapts a function to Provider (useful in tests).
type ProviderFunc func(key string) string

func (f ProviderFunc) GetSecret(key string) string { return f(key) }

var current Provider = EnvProvider{}

// SetProvider replaces the global secret provider. Returns a restore function.
func SetProvider(p Provider) func() {
	prev := current
	current = p
	return func() { current = prev }
}

// GetSecret returns a secret value from the active provider.
func GetSecret(key string) string {
	return current.GetSecret(key)
}

// DBEncryptionKey returns the AES-256-GCM master key.
func DBEncryptionKey() string {
	return GetSecret(KeyDBEncryption)
}

// JWTSecret returns the JWT signing secret.
func JWTSecret() string {
	return GetSecret(KeyJWTSecret)
}

// RequireDBEncryptionKey returns the master key or an error if misconfigured.
func RequireDBEncryptionKey() (string, error) {
	key := strings.TrimSpace(DBEncryptionKey())
	if len(key) != 32 {
		return "", fmt.Errorf("%s must be 32 characters", KeyDBEncryption)
	}
	return key, nil
}
