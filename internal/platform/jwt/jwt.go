package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims = jwt.MapClaims

func New(secret string, claims Claims, ttl time.Duration) (string, error) {
	now := time.Now()
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(ttl).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func Verify(secret, tokenStr string) (Claims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}
