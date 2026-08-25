package security

import (
	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"golang.org/x/crypto/bcrypt"
)

const minPasswordLength = 10

func HashPassword(password string) (string, error) {
	if len(password) < minPasswordLength {
		return "", apperr.New(apperr.CodeInvalid, "password must contain at least 10 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, "hash password", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return apperr.New(apperr.CodeUnauthorized, "email or password is incorrect")
	}
	return nil
}
