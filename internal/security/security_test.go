package security

import (
	"strings"
	"testing"
)

func TestHashPasswordAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "correct-horse-battery" {
		t.Fatal("password was stored in clear text")
	}
	if err := CheckPassword(hash, "correct-horse-battery"); err != nil {
		t.Fatalf("CheckPassword(correct) error = %v", err)
	}
	if err := CheckPassword(hash, "another-password"); err == nil {
		t.Fatal("CheckPassword(wrong) succeeded")
	}
}

func TestHashPasswordRejectsShortInput(t *testing.T) {
	for _, password := range []string{"", "short", "123456789"} {
		t.Run(password, func(t *testing.T) {
			if _, err := HashPassword(password); err == nil {
				t.Fatalf("HashPassword(%q) succeeded", password)
			}
		})
	}
}

func TestPasswordHashesUseIndependentSalt(t *testing.T) {
	first, err := HashPassword("same-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword("same-password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("bcrypt hashes should use independent salts")
	}
	if err := CheckPassword(first, "same-password"); err != nil {
		t.Fatal(err)
	}
	if err := CheckPassword(second, "same-password"); err != nil {
		t.Fatal(err)
	}
}

func TestRandomIDsHavePrefixAndAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for index := 0; index < 100; index++ {
		id, err := RandomID("coh")
		if err != nil {
			t.Fatalf("RandomID() error = %v", err)
		}
		if !strings.HasPrefix(id, "coh_") {
			t.Fatalf("id = %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestTokenStoresOnlyHash(t *testing.T) {
	plain, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	if plain == "" || hash == "" || plain == hash {
		t.Fatalf("plain/hash pair is invalid: %q %q", plain, hash)
	}
	if got := HashToken(plain); got != hash {
		t.Fatalf("HashToken() = %q, want %q", got, hash)
	}
	if len(hash) != 64 {
		t.Fatalf("hash length = %d", len(hash))
	}
}

func TestPayloadHashIsStableAndSensitive(t *testing.T) {
	first := HashPayload([]byte(`{"cohort":"a","count":20}`))
	second := HashPayload([]byte(`{"cohort":"a","count":20}`))
	changed := HashPayload([]byte(`{"cohort":"a","count":21}`))
	if first != second {
		t.Fatal("same payload produced different hashes")
	}
	if first == changed {
		t.Fatal("different payload produced same hash")
	}
}
