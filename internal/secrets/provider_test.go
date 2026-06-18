package secrets

import (
	"testing"

	"aigent/internal/utils"
)

func TestEnvProviderGetSecret(t *testing.T) {
	t.Setenv(KeyDBEncryption, "12345678901234567890123456789012")
	restore := SetProvider(EnvProvider{})
	defer restore()

	if got := GetSecret(KeyDBEncryption); got != "12345678901234567890123456789012" {
		t.Fatalf("GetSecret = %q", got)
	}
}

func TestProviderFuncOverride(t *testing.T) {
	restore := SetProvider(ProviderFunc(func(key string) string {
		if key == KeyDBEncryption {
			return "abcdef0123456789abcdef0123456789"
		}
		return "ignored"
	}))
	defer restore()

	key, err := RequireDBEncryptionKey()
	if err != nil {
		t.Fatalf("RequireDBEncryptionKey: %v", err)
	}
	if key != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("unexpected key %q", key)
	}
}

func TestRequireDBEncryptionKeyLength(t *testing.T) {
	restore := SetProvider(ProviderFunc(func(string) string { return "short" }))
	defer restore()

	if _, err := RequireDBEncryptionKey(); err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestEncryptDecryptViaAdapterKey(t *testing.T) {
	const key = "12345678901234567890123456789012"
	restore := SetProvider(ProviderFunc(func(k string) string {
		if k == KeyDBEncryption {
			return key
		}
		return ""
	}))
	defer restore()

	masterKey, err := RequireDBEncryptionKey()
	if err != nil {
		t.Fatalf("RequireDBEncryptionKey: %v", err)
	}

	plaintext := "provider-api-key-value"
	encrypted, err := utils.Encrypt(plaintext, masterKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	decrypted, err := utils.Decrypt(encrypted, masterKey)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("round-trip mismatch: %q", decrypted)
	}
}

func TestUnconfiguredVaultStub(t *testing.T) {
	var vp VaultProvider = UnconfiguredVault{}
	if vp.Ready() {
		t.Fatal("expected vault stub to be not ready")
	}
	if vp.GetSecret(KeyDBEncryption) != "" {
		t.Fatal("expected empty secret from unconfigured vault")
	}
}
