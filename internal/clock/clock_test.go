package clock

import (
	"sync"
	"testing"
	"time"
)

func TestFakeNormalizesAndAdvancesUTC(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	start := time.Date(2026, 8, 25, 9, 30, 0, 0, location)
	clock := NewFake(start)
	if got := clock.Now().Location(); got != time.UTC {
		t.Fatalf("location = %v, want UTC", got)
	}
	clock.Advance(90 * time.Minute)
	want := start.UTC().Add(90 * time.Minute)
	if got := clock.Now(); !got.Equal(want) {
		t.Fatalf("Now() = %v, want %v", got, want)
	}
}

func TestFakeSetReplacesTime(t *testing.T) {
	clock := NewFake(time.Unix(0, 0))
	want := time.Date(2026, 12, 1, 1, 2, 3, 4, time.UTC)
	clock.Set(want)
	if got := clock.Now(); got != want {
		t.Fatalf("Now() = %v, want %v", got, want)
	}
}

func TestFakeSupportsConcurrentReadersAndWriter(t *testing.T) {
	clock := NewFake(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := 0; index < 100; index++ {
				_ = clock.Now()
			}
		}()
	}
	for index := 0; index < 100; index++ {
		clock.Advance(time.Second)
	}
	wait.Wait()
	if got := clock.Now(); got.Second() != 40 || got.Minute() != 1 {
		t.Fatalf("final time = %v", got)
	}
}

func TestRealReturnsUTC(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	got := (Real{}).Now()
	after := time.Now().UTC().Add(time.Second)
	if got.Location() != time.UTC {
		t.Fatalf("location = %v", got.Location())
	}
	if got.Before(before) || got.After(after) {
		t.Fatalf("real time %v outside expected range", got)
	}
}
