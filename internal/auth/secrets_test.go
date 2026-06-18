package auth

import (
	"testing"
)

func setTestSecrets(t *testing.T) {
	t.Helper()
	t.Setenv("DB_ENCRYPTION_KEY", "12345678901234567890123456789012")
	t.Setenv("JWT_SECRET", "test_jwt_secret_16chars")
}

func TestValidateStartupSecrets(t *testing.T) {
	setTestSecrets(t)
	if err := ValidateStartupSecrets(); err != nil {
		t.Fatalf("ValidateStartupSecrets: %v", err)
	}
}

func TestValidateDBEncryptionKeyLength(t *testing.T) {
	if err := ValidateDBEncryptionKey("short"); err == nil {
		t.Fatal("expected error for short DB_ENCRYPTION_KEY")
	}
	if err := ValidateDBEncryptionKey("12345678901234567890123456789012"); err != nil {
		t.Fatalf("expected valid key, got %v", err)
	}
}

func TestValidateJWTSecretRequired(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if err := ValidateJWTSecret(""); err == nil {
		t.Fatal("expected error for empty JWT_SECRET")
	}
	if err := ValidateJWTSecret("short"); err == nil {
		t.Fatal("expected error for JWT_SECRET shorter than minimum")
	}
	if err := ValidateJWTSecret("valid_jwt_secret"); err != nil {
		t.Fatalf("expected valid secret, got %v", err)
	}
}

func TestJWTUsesJWTSecretNotEncryptionKey(t *testing.T) {
	t.Setenv("DB_ENCRYPTION_KEY", "12345678901234567890123456789012")
	t.Setenv("JWT_SECRET", "dedicated_jwt_signing_secret")

	token, err := GenerateToken(7, "alice", []string{"operator"})
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, err := ValidateToken(token); err != nil {
		t.Fatalf("ValidateToken with JWT_SECRET: %v", err)
	}

	wrongKey := []byte("12345678901234567890123456789012")
	if _, err := jwtParseWithKey(token, wrongKey); err == nil {
		t.Fatal("expected token signed with DB_ENCRYPTION_KEY to fail validation")
	}
}
