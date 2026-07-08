package jwt

import (
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

type JWT struct {
	secret   []byte
	duration time.Duration
}

type Claims struct {
	SessionID string `json:"session_id"`
	jwtlib.RegisteredClaims
}

func New(secret string, duration time.Duration) *JWT {
	return &JWT{secret: []byte(secret), duration: duration}
}

func (j *JWT) Sign(sessionID string) (string, error) {
	claims := Claims{
		SessionID: sessionID,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(j.duration)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("SignedString: %w", err)
	}
	return signed, nil
}

func (j *JWT) Verify(tokenString string) (string, error) {
	token, err := jwtlib.ParseWithClaims(tokenString, &Claims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return "", fmt.Errorf("ParseWithClaims: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	return claims.SessionID, nil
}
