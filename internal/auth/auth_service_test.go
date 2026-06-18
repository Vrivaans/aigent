package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndValidateTokenWithClaims(t *testing.T) {
	t.Setenv("DB_ENCRYPTION_KEY", "12345678901234567890123456789012")

	roles := []string{"admin", "operator"}
	token, err := GenerateToken(42, "admin", roles)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("expected user_id 42, got %d", claims.UserID)
	}
	if claims.Username != "admin" {
		t.Fatalf("expected username admin, got %q", claims.Username)
	}
	if len(claims.Roles) != 2 || claims.Roles[0] != "admin" {
		t.Fatalf("unexpected roles: %v", claims.Roles)
	}
}

func TestValidateTokenRejectsMissingUserID(t *testing.T) {
	t.Setenv("DB_ENCRYPTION_KEY", "12345678901234567890123456789012")

	legacy := &Claims{
		UserID:   0,
		Username: "legacy",
		Roles:    []string{},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy)
	tokenString, err := token.SignedString(jwtSecretKey())
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}
	if _, err := ValidateToken(tokenString); err == nil {
		t.Fatal("expected error for token without user_id")
	}
}

func TestGenerateTokenRequiresUserID(t *testing.T) {
	t.Setenv("DB_ENCRYPTION_KEY", "12345678901234567890123456789012")

	if _, err := GenerateToken(0, "admin", nil); err == nil {
		t.Fatal("expected error when user_id is zero")
	}
}
