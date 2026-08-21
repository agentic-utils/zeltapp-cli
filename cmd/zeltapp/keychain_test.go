package main

import (
	"os"
	"testing"
)

// Exercises the real macOS `security` roundtrip: set, read, set again
// (idempotency), read, delete. Uses a throwaway account so it never touches the
// real zeltapp-cli entry. Skips where /usr/bin/security is absent (e.g. Linux CI).
func TestKeychain_SetGetIdempotent(t *testing.T) {
	const bin = "/usr/bin/security"
	if info, err := os.Stat(bin); err != nil || info.Mode()&0o111 == 0 {
		t.Skip("no macOS security binary")
	}
	s := &fileStore{keychainCmd: bin}
	email := "zeltapp-cli-test-" + t.Name() + "@example.com"
	t.Cleanup(func() { _ = s.DeletePassword(email) })

	// Password with a space and symbols - the kind that broke single-line input.
	pw1 := "p@ss w0rd!#1"
	if err := s.SetPassword(email, pw1); err != nil {
		t.Fatalf("first set: %v", err)
	}
	if got, err := s.GetPassword(email); err != nil || got != pw1 {
		t.Fatalf("get after first set: got %q err %v", got, err)
	}

	// Second set on an existing entry must succeed and overwrite (this is the
	// path that used to fail with "already exists").
	pw2 := "different-2"
	if err := s.SetPassword(email, pw2); err != nil {
		t.Fatalf("second set (idempotency): %v", err)
	}
	if got, err := s.GetPassword(email); err != nil || got != pw2 {
		t.Fatalf("get after second set: got %q err %v", got, err)
	}
}
