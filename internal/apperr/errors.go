package apperr

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeInvalid      Code = "invalid_argument"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeExpired      Code = "expired"
	CodeUnavailable  Code = "unavailable"
	CodeInternal     Code = "internal"
)

type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func (e *Error) Unwrap() error { return e.Cause }

func New(code Code, message string) error {
	return &Error{Code: code, Message: message}
}

func Wrap(code Code, message string, cause error) error {
	if cause == nil {
		return nil
	}
	return &Error{Code: code, Message: message, Cause: cause}
}

func CodeOf(err error) Code {
	var target *Error
	if errors.As(err, &target) {
		return target.Code
	}
	return CodeInternal
}

func MessageOf(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Message
	}
	return "internal server error"
}

func IsCode(err error, code Code) bool { return CodeOf(err) == code }
