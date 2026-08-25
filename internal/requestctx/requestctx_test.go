package requestctx

import (
	"context"
	"testing"
)

func TestRequestIDDefaultsToEmpty(t *testing.T) {
	if got := RequestID(context.Background()); got != "" {
		t.Fatalf("RequestID(background) = %q", got)
	}
}

func TestRequestIDRoundTripsWithoutChangingParent(t *testing.T) {
	parent := context.Background()
	child := WithRequestID(parent, "req-42")
	if got := RequestID(child); got != "req-42" {
		t.Fatalf("RequestID(child) = %q", got)
	}
	if got := RequestID(parent); got != "" {
		t.Fatalf("RequestID(parent) = %q", got)
	}
}

func TestNestedRequestIDUsesNewestValue(t *testing.T) {
	first := WithRequestID(context.Background(), "req-first")
	second := WithRequestID(first, "req-second")
	if got := RequestID(second); got != "req-second" {
		t.Fatalf("RequestID(second) = %q", got)
	}
	if got := RequestID(first); got != "req-first" {
		t.Fatalf("RequestID(first) = %q", got)
	}
}
