package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStore_SessionRoundtrip(t *testing.T) {
	dir := t.TempDir()
	store := &fileStore{dir: dir}

	s := &session{
		Email: "x@example.com", UserID: 42, CompanyID: 1,
		DisplayName: "X", Token: "T", RefreshToken: "R",
	}
	if err := store.SaveSession(s); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected 0600, got %v", info.Mode().Perm())
	}
	loaded, err := store.LoadSession()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Email != s.Email || loaded.UserID != s.UserID || loaded.Token != s.Token {
		t.Errorf("roundtrip lost data: %#v", loaded)
	}
	if loaded.SavedAt.IsZero() {
		t.Error("SavedAt not stamped")
	}
}

func TestFileStore_ClearMissing(t *testing.T) {
	dir := t.TempDir()
	store := &fileStore{dir: dir}
	// Clearing when nothing exists should not error.
	if err := store.ClearSession(); err != nil {
		t.Errorf("clear on missing should be no-op, got %v", err)
	}
}

func TestFileStore_ClearRemovesFile(t *testing.T) {
	dir := t.TempDir()
	store := &fileStore{dir: dir}
	_ = store.SaveSession(&session{Email: "x"})
	if err := store.ClearSession(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "session.json")); !os.IsNotExist(err) {
		t.Error("session.json still exists after Clear")
	}
}

func TestFileStore_LoadMissingReturnsError(t *testing.T) {
	dir := t.TempDir()
	store := &fileStore{dir: dir}
	if _, err := store.LoadSession(); err == nil {
		t.Error("expected error loading missing session")
	}
}

func TestFileStore_SaveAtTimestampUpdated(t *testing.T) {
	dir := t.TempDir()
	store := &fileStore{dir: dir}
	s := &session{Email: "x"}
	if err := store.SaveSession(s); err != nil {
		t.Fatal(err)
	}
	t1 := s.SavedAt
	time.Sleep(2 * time.Millisecond)
	if err := store.SaveSession(s); err != nil {
		t.Fatal(err)
	}
	if !s.SavedAt.After(t1) {
		t.Error("SavedAt should advance on resave")
	}
}

func TestFileStore_KeychainUnavailable(t *testing.T) {
	// keychainCmd empty => all keychain ops should error gracefully.
	store := &fileStore{dir: t.TempDir(), keychainCmd: ""}
	if _, err := store.GetPassword("x"); err == nil {
		t.Error("Get should error when keychain unavailable")
	}
	if err := store.SetPassword("x", "y"); err == nil {
		t.Error("Set should error when keychain unavailable")
	}
	if err := store.DeletePassword("x"); err == nil {
		t.Error("Delete should error when keychain unavailable")
	}
}

func TestMemStore_RoundtripAndCrypto(t *testing.T) {
	m := newMemStore()
	if _, err := m.LoadSession(); err == nil {
		t.Error("empty store should error")
	}
	s := &session{Email: "y", Token: "T"}
	if err := m.SaveSession(s); err != nil {
		t.Fatal(err)
	}
	loaded, err := m.LoadSession()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != "T" {
		t.Error("token lost")
	}
	if err := m.SetPassword("y", "pw"); err != nil {
		t.Fatal(err)
	}
	pw, err := m.GetPassword("y")
	if err != nil || pw != "pw" {
		t.Errorf("password roundtrip failed: pw=%s err=%v", pw, err)
	}
	if err := m.DeletePassword("y"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetPassword("y"); err == nil {
		t.Error("password should be gone after Delete")
	}
	if err := m.ClearSession(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LoadSession(); err == nil {
		t.Error("session should be gone after Clear")
	}
}
