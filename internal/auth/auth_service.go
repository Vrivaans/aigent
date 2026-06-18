package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func jwtSecretKey() []byte {
	return JWTSecretKey()
}

type Claims struct {
	UserID   uint     `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

func GenerateToken(userID uint, username string, roles []string) (string, error) {
	if userID == 0 {
		return "", errors.New("user_id is required")
	}
	if username == "" {
		return "", errors.New("username is required")
	}
	if roles == nil {
		roles = []string{}
	}

	claims := &Claims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecretKey())
}

func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecretKey(), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		if claims.UserID == 0 {
			return nil, errors.New("invalid token: missing user_id")
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
