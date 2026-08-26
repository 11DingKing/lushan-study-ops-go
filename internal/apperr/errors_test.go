package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewPreservesCodeAndPublicMessage(t *testing.T) {
	err := New(CodeConflict, "resource is already held")
	if got := CodeOf(err); got != CodeConflict {
		t.Fatalf("CodeOf() = %q, want %q", got, CodeConflict)
	}
	if got := MessageOf(err); got != "resource is already held" {
		t.Fatalf("MessageOf() = %q", got)
	}
	if err.Error() != "resource is already held" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestWrapRetainsCauseForErrorsIs(t *testing.T) {
	cause := errors.New("database locked")
	err := Wrap(CodeUnavailable, "persist confirmation", cause)
	if !errors.Is(err, cause) {
		t.Fatal("wrapped error does not retain original cause")
	}
	if got := err.Error(); got != "persist confirmation: database locked" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestWrapNilReturnsNil(t *testing.T) {
	if err := Wrap(CodeInternal, "unused", nil); err != nil {
		t.Fatalf("Wrap(nil) = %v", err)
	}
}

func TestUnknownErrorUsesSafeDefaults(t *testing.T) {
	err := fmt.Errorf("raw driver detail")
	if got := CodeOf(err); got != CodeInternal {
		t.Fatalf("CodeOf(raw) = %q", got)
	}
	if got := MessageOf(err); got != "internal server error" {
		t.Fatalf("MessageOf(raw) = %q", got)
	}
}

func TestIsCodeTraversesSeveralWrapLayers(t *testing.T) {
	err := Wrap(CodeExpired, "session expired", errors.New("deadline"))
	err = fmt.Errorf("authenticate: %w", err)
	err = fmt.Errorf("middleware: %w", err)
	if !IsCode(err, CodeExpired) {
		t.Fatalf("IsCode(%v, expired) = false", err)
	}
	if IsCode(err, CodeUnauthorized) {
		t.Fatalf("IsCode(%v, unauthorized) = true", err)
	}
}

func TestEveryCodeRoundTrips(t *testing.T) {
	cases := []Code{
		CodeInvalid,
		CodeUnauthorized,
		CodeForbidden,
		CodeNotFound,
		CodeConflict,
		CodeExpired,
		CodeUnavailable,
		CodeInternal,
	}
	for _, code := range cases {
		t.Run(string(code), func(t *testing.T) {
			err := New(code, "public")
			if CodeOf(err) != code {
				t.Fatalf("round trip code = %q", CodeOf(err))
			}
		})
	}
}
