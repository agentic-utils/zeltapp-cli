package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Default per-endpoint TTLs. The map keys are exact paths (no query string);
// anything not listed is uncached unless explicitly requested.
var cacheTTLs = map[string]time.Duration{
	"/apiv2/users/cache":                       5 * time.Minute,
	"/apiv2/companies/config":                  1 * time.Hour,
	"/apiv2/companies/general-settings":        1 * time.Hour,
	"/apiv2/companies/departments":             1 * time.Hour,
	"/apiv2/companies/sites":                   1 * time.Hour,
	"/apiv2/companies/public-url":              24 * time.Hour,
	"/apiv2/job-positions":                     1 * time.Hour,
	"/apiv2/absence-policies/team":             1 * time.Hour,
	"/apiv2/absence-policies/team/extended":    1 * time.Hour,
	"/apiv2/company/forms":                     1 * time.Hour,
	"/apiv2/company/fields/all-fields-profile": 1 * time.Hour,
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

func cachePath(path string) string {
	h := sha256.Sum256([]byte(path))
	return filepath.Join(cacheDir(), hex.EncodeToString(h[:])+".json")
}

// cacheTTLFor returns the TTL for a given path, or 0 if uncached.
func cacheTTLFor(path string) time.Duration {
	return cacheTTLs[path]
}

func cacheGet(path string) (json.RawMessage, bool) {
	if flagNoCache {
		return nil, false
	}
	b, err := os.ReadFile(cachePath(path))
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

func cacheSet(path string, body json.RawMessage, ttl time.Duration) error {
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
	return os.WriteFile(cachePath(path), b, 0o600)
}

// cacheClear deletes the entire cache directory.
func cacheClear() error {
	err := os.RemoveAll(cacheDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
