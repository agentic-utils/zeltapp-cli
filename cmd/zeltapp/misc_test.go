package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	u := parseURL("https://example.com/path?q=1")
	if u == nil {
		t.Fatal("nil URL")
	}
	if u.Host != "example.com" {
		t.Errorf("host wrong: %s", u.Host)
	}
}

func TestMustURL(t *testing.T) {
	u := mustURL("https://go.zelt.app")
	if u == nil || u.Host != "go.zelt.app" {
		t.Errorf("mustURL wrong: %v", u)
	}
}

func TestLocalTimezone(t *testing.T) {
	t.Setenv("TZ", "Europe/London")
	if got := localTimezone(); got != "Europe/London" {
		t.Errorf("expected Europe/London, got %s", got)
	}
}

func TestPromptMFAFromStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("123456\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	code, err := promptMFA("email")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if code != "123456" {
		t.Errorf("expected 123456, got %q", code)
	}
}

func TestKeychain_RoundtripIfAvailable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	if _, err := os.Stat("/usr/bin/security"); err != nil {
		t.Skip("/usr/bin/security not available")
	}
	store := &fileStore{dir: t.TempDir(), keychainCmd: "/usr/bin/security"}
	// Use a unique email so we don't collide with anything the user has stored.
	email := "zeltapp-cli-test+" + t.Name() + "@example.invalid"
	password := "test-password-" + t.Name()

	t.Cleanup(func() { _ = store.DeletePassword(email) })

	if err := store.SetPassword(email, password); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := store.GetPassword(email)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != password {
		t.Errorf("expected %q, got %q", password, got)
	}
	if err := store.DeletePassword(email); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := store.GetPassword(email); err == nil {
		t.Error("password should be gone after delete")
	}
}

func TestKeychain_GetReturnsNotInKeychain(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	if _, err := os.Stat("/usr/bin/security"); err != nil {
		t.Skip("/usr/bin/security not available")
	}
	store := &fileStore{dir: t.TempDir(), keychainCmd: "/usr/bin/security"}
	_, err := store.GetPassword("definitely-not-in-keychain-" + t.Name() + "@example.invalid")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not in keychain") {
		t.Errorf("expected 'not in keychain' message, got: %v", err)
	}
}

func TestDefaultStoreDetectsKeychainOnDarwin(t *testing.T) {
	store := defaultStore()
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/usr/bin/security"); err == nil {
			if store.keychainCmd == "" {
				t.Error("expected keychain detection on darwin")
			}
		}
	}
}
