package hs

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims[T any] struct {
	User *T
	jwt.RegisteredClaims
}

func jwtSecret() []byte {
	s := os.Getenv("ADMIN_JWT_SECRET")
	if s == "" {
		s = "admin-service-secret"
	}
	return []byte(s)
}

// GenerateJWT creates a signed JWT token for a user
func GenerateJWT[T any](user *T, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := JWTClaims[T]{
		User: user,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "admin-service",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

// ParseJWT verifies token and returns claims
func ParseJWT[T any](tokenString string) (*JWTClaims[T], error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims[T]{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*JWTClaims[T]); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
