package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Default per-endpoint TTLs. Only paths the CLI actually calls are listed
// (review #6: don't keep aspirational entries — they confuse the next reader).
var cacheTTLs = map[string]time.Duration{
	"/apiv2/users/cache":                    5 * time.Minute,
	"/apiv2/companies/config":               1 * time.Hour,
	"/apiv2/companies/general-settings":     1 * time.Hour,
	"/apiv2/companies/departments":          1 * time.Hour,
	"/apiv2/companies/sites":                1 * time.Hour,
	"/apiv2/job-positions":                  1 * time.Hour,
	"/apiv2/absence-policies/team/extended": 1 * time.Hour,
}

type cacheEntry struct {
	Path      string          `json:"path"`
	StoredAt  time.Time       `json:"storedAt"`
	ExpiresAt time.Time       `json:"expiresAt"`
	Body      json.RawMessage `json:"body"`
}

// flagNoCache is the global opt-out flag (registered in main.go).
var flagNoCache bool

// cacheDir returns the cache directory, creating it on demand.
func cacheDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "zeltapp-cli", "cache")
}

// cacheKey scopes the on-disk key by the calling identity so that logging out
// and back in as a different user (or a contractor switching companies) does
// not surface the previous user's directory / org metadata.
func cacheKey(scope, path string) string {
	h := sha256.Sum256([]byte(scope + "|" + path))
	return hex.EncodeToString(h[:])
}

func cachePath(scope, path string) string {
	return filepath.Join(cacheDir(), cacheKey(scope, path)+".json")
}

// cacheTTLFor returns the TTL for a given path, or 0 if uncached.
func cacheTTLFor(path string) time.Duration {
	return cacheTTLs[path]
}

func cacheGet(scope, path string) (json.RawMessage, bool) {
	if flagNoCache {
		return nil, false
	}
	b, err := os.ReadFile(cachePath(scope, path))
	if err != nil {
		return nil, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(b, &entry); err != nil {
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Body, true
}

func cacheSet(scope, path string, body json.RawMessage, ttl time.Duration) error {
	if flagNoCache || ttl == 0 {
		return nil
	}
	if err := os.MkdirAll(cacheDir(), 0o700); err != nil {
		return err
	}
	entry := cacheEntry{
		Path:      path,
		StoredAt:  time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Body:      body,
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return writeFileAtomic(cachePath(scope, path), b, 0o600)
}

// cacheClear removes the cache directory contents. Safe across symlinks
// (review #14): it refuses to recurse into a symlink rather than nuking the
// target.
func cacheClear() error {
	dir := cacheDir()
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to clear cache: %s is a symlink", dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return nil
}

// cacheScope returns the identity prefix used by cachePath. Empty for unset
// sessions (still partitions from authed scopes).
func cacheScope(s *session) string {
	if s == nil {
		return "anon"
	}
	return fmt.Sprintf("u:%d:c:%d", s.UserID, s.CompanyID)
}
