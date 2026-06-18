package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func jwtParseWithKey(tokenString string, key []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return key, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

func TestGenerateAndValidateTokenWithClaims(t *testing.T) {
	setTestSecrets(t)

	roles := []string{"admin", "operator"}
	token, err := GenerateToken(42, "admin", roles, 7)
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
	if claims.TenantID != 7 {
		t.Fatalf("expected tenant_id 7, got %d", claims.TenantID)
	}
}

func TestValidateTokenRejectsMissingUserID(t *testing.T) {
	setTestSecrets(t)

	legacy := &Claims{
		UserID:   0,
		Username: "legacy",
		Roles:    []string{},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy)
	tokenString, err := token.SignedString(JWTSecretKey())
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}
	if _, err := ValidateToken(tokenString); err == nil {
		t.Fatal("expected error for token without user_id")
	}
}

func TestGenerateTokenRequiresUserID(t *testing.T) {
	setTestSecrets(t)

	if _, err := GenerateToken(0, "admin", nil, 1); err == nil {
		t.Fatal("expected error when user_id is zero")
	}
}
