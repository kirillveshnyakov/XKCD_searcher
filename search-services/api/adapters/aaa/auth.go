package aaa

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	secretKey = "something secret here" // token sign key
	adminRole = "superuser"             // token subject
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrWrongPassword = errors.New("wrong password")
	ErrInvalidToken  = errors.New("invalid token")
	ErrWrongSubject  = errors.New("wrong subject")
)

// AAA Authentication, Authorization, Accounting
type AAA struct {
	users    map[string]string
	tokenTTL time.Duration
	log      *slog.Logger
}

func New(tokenTTL time.Duration, log *slog.Logger) (AAA, error) {
	const adminUser = "ADMIN_USER"
	const adminPass = "ADMIN_PASSWORD"
	user, ok := os.LookupEnv(adminUser)
	if !ok {
		return AAA{}, fmt.Errorf("could not get admin user from enviroment")
	}
	password, ok := os.LookupEnv(adminPass)
	if !ok {
		return AAA{}, fmt.Errorf("could not get admin password from enviroment")
	}

	return AAA{
		users:    map[string]string{user: password},
		tokenTTL: tokenTTL,
		log:      log,
	}, nil
}

func (a AAA) Login(name, password string) (string, error) {
	savedPassword, ok := a.users[name]
	if !ok {
		return "", ErrUserNotFound
	}
	if password != savedPassword {
		return "", ErrWrongPassword
	}

	claims := jwt.RegisteredClaims{
		Subject:   adminRole,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.tokenTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secretKey))
}

func (a AAA) Verify(tokenString string) error {
	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}

		return []byte(secretKey), nil
	})
	if err != nil {
		return fmt.Errorf("parse token: %w", err)
	}

	if !token.Valid {
		a.log.Error("invalid token")
		return ErrInvalidToken
	}

	if claims.Subject != adminRole {
		a.log.Error("wrong subject", "subject", claims.Subject)
		return ErrWrongSubject
	}

	return nil
}
