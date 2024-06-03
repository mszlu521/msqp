package jwts

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	Uid string `json:"uid"`
	jwt.RegisteredClaims
}

func GenToken(claims *CustomClaims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(token, secret string) (string, error) {
	t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	if claims, ok := t.Claims.(jwt.MapClaims); ok && t.Valid {
		return fmt.Sprintf("%v", claims["uid"]), nil
	}
	return "", errors.New("token not valid")
}
