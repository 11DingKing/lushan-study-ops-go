package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/storage"
)

func idempotencyFixture(t *testing.T) (*Service, *storage.Store, *time.Time) {
	t.Helper()
	store, err := storage.OpenMemory(context.Background(), t.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	service := &Service{Repo: store, TTL: time.Hour, Now: func() time.Time { return now }}
	return service, store, &now
}

func TestDoPersistsAndReplaysOriginalOutcome(t *testing.T) {
	service, _, _ := idempotencyFixture(t)
	calls := 0
	operation := func(context.Context) (int, []byte, error) {
		calls++
		return 201, []byte(`{"id":"coh-1"}`), nil
	}
	for index := 0; index < 2; index++ {
		status, response, err := service.Do(context.Background(), "leader:POST:/v1/applications", "key-1",
			[]byte(`{"name":"group"}`), operation)
		if err != nil {
			t.Fatalf("Do(%d) error = %v", index, err)
		}
		if status != 201 || string(response) != `{"id":"coh-1"}` {
			t.Fatalf("outcome = %d %s", status, response)
		}
		response[0] = 'x'
	}
	if calls != 1 {
		t.Fatalf("operation calls = %d", calls)
	}
}

func TestDoRejectsPayloadAliasForActiveKey(t *testing.T) {
	service, _, _ := idempotencyFixture(t)
	operation := func(context.Context) (int, []byte, error) { return 200, []byte("first"), nil }
	if _, _, err := service.Do(context.Background(), "scope", "key", []byte("payload-a"), operation); err != nil {
		t.Fatal(err)
	}
	called := false
	_, _, err := service.Do(context.Background(), "scope", "key", []byte("payload-b"),
		func(context.Context) (int, []byte, error) {
			called = true
			return 200, []byte("second"), nil
		})
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("payload alias error = %v", err)
	}
	if called {
		t.Fatal("conflicting operation was invoked")
	}
}

func TestDoDoesNotPersistFailedOperation(t *testing.T) {
	service, _, _ := idempotencyFixture(t)
	want := errors.New("capacity changed")
	_, _, err := service.Do(context.Background(), "scope", "key", []byte("payload"),
		func(context.Context) (int, []byte, error) { return 0, nil, want })
	if !errors.Is(err, want) {
		t.Fatalf("Do() error = %v", err)
	}
	calls := 0
	status, body, err := service.Do(context.Background(), "scope", "key", []byte("payload"),
		func(context.Context) (int, []byte, error) {
			calls++
			return 202, []byte("accepted"), nil
		})
	if err != nil || status != 202 || string(body) != "accepted" || calls != 1 {
		t.Fatalf("retry outcome = %d %q %v calls=%d", status, body, err, calls)
	}
}

func TestDoAllowsKeyReuseAfterExpiry(t *testing.T) {
	service, _, now := idempotencyFixture(t)
	firstCalls := 0
	if _, _, err := service.Do(context.Background(), "scope", "key", []byte("first"),
		func(context.Context) (int, []byte, error) {
			firstCalls++
			return 200, []byte("one"), nil
		}); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(2 * time.Hour)
	secondCalls := 0
	status, response, err := service.Do(context.Background(), "scope", "key", []byte("second"),
		func(context.Context) (int, []byte, error) {
			secondCalls++
			return 201, []byte("two"), nil
		})
	if err != nil {
		t.Fatalf("reuse error = %v", err)
	}
	if firstCalls != 1 || secondCalls != 1 || status != 201 || string(response) != "two" {
		t.Fatalf("reuse outcome calls=%d/%d status=%d body=%s", firstCalls, secondCalls, status, response)
	}
}

func TestDoRequiresKey(t *testing.T) {
	service, _, _ := idempotencyFixture(t)
	_, _, err := service.Do(context.Background(), "scope", "", []byte("payload"),
		func(context.Context) (int, []byte, error) { return 200, nil, nil })
	if !apperr.IsCode(err, apperr.CodeInvalid) {
		t.Fatalf("missing key error = %v", err)
	}
}
