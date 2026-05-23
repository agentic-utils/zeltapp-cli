package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withCacheTempDir points cacheDir() at a temp dir for the duration of one test
// and re-enables caching (TestMain disables it by default).
func withCacheTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	prevNoCache := flagNoCache
	flagNoCache = false
	t.Cleanup(func() { flagNoCache = prevNoCache })
	return filepath.Join(dir, "zeltapp-cli", "cache")
}

func TestCache_RoundtripFile(t *testing.T) {
	cacheDirPath := withCacheTempDir(t)
	path := "/apiv2/users/cache"
	body := json.RawMessage(`{"x":1}`)
	if err := cacheSet(path, body, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	got, ok := cacheGet(path)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got) != string(body) {
		t.Errorf("body changed: %s vs %s", got, body)
	}
	// File should have 0600 permissions for security.
	files, _ := os.ReadDir(cacheDirPath)
	if len(files) != 1 {
		t.Fatalf("expected 1 cache file, got %d", len(files))
	}
	info, _ := os.Stat(filepath.Join(cacheDirPath, files[0].Name()))
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected 0600 perms, got %v", info.Mode().Perm())
	}
}

func TestCache_ExpiresOnTTL(t *testing.T) {
	withCacheTempDir(t)
	path := "/apiv2/users/cache"
	body := json.RawMessage(`{"x":1}`)
	// Set with a TTL already in the past.
	if err := cacheSet(path, body, -1*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, ok := cacheGet(path); ok {
		t.Error("expected miss for expired entry")
	}
}

func TestCache_DisabledByFlag(t *testing.T) {
	withCacheTempDir(t)
	flagNoCache = true
	defer func() { flagNoCache = false }()
	path := "/apiv2/users/cache"
	body := json.RawMessage(`{"x":1}`)
	// Set should noop when flag is on.
	if err := cacheSet(path, body, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, ok := cacheGet(path); ok {
		t.Error("expected miss when flagNoCache=true")
	}
}

func TestCache_UncachedPathNoOp(t *testing.T) {
	if cacheTTLFor("/apiv2/users/6380/basic") != 0 {
		t.Error("user-specific paths should not be cached")
	}
	if cacheTTLFor("/apiv2/auth/me") != 0 {
		t.Error("auth/me should not be cached (changes too often)")
	}
}

func TestCache_KnownPathsCached(t *testing.T) {
	for _, p := range []string{
		"/apiv2/users/cache",
		"/apiv2/companies/config",
		"/apiv2/companies/departments",
		"/apiv2/job-positions",
		"/apiv2/absence-policies/team/extended",
	} {
		if cacheTTLFor(p) == 0 {
			t.Errorf("%s should be cacheable", p)
		}
	}
}

func TestCache_ClientHitAvoidsNetwork(t *testing.T) {
	withCacheTempDir(t)
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/users/cache", 200, []byte(`[]`))
	c, _ := newAuthedTestClient(t, srv)

	// First call hits network.
	var out1 []any
	if err := c.do("GET", "/apiv2/users/cache", nil, &out1); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("GET", "/apiv2/users/cache")) != 1 {
		t.Errorf("expected 1 network call, got %d", len(st.requestsTo("GET", "/apiv2/users/cache")))
	}
	// Second call should hit cache (no new network request).
	var out2 []any
	if err := c.do("GET", "/apiv2/users/cache", nil, &out2); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("GET", "/apiv2/users/cache")) != 1 {
		t.Errorf("expected still 1 network call (2nd from cache), got %d",
			len(st.requestsTo("GET", "/apiv2/users/cache")))
	}
}

func TestCache_NoCacheFlagBypassesCache(t *testing.T) {
	withCacheTempDir(t)
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/users/cache", 200, []byte(`[]`))
	c, _ := newAuthedTestClient(t, srv)

	// Prime the cache.
	var out []any
	if err := c.do("GET", "/apiv2/users/cache", nil, &out); err != nil {
		t.Fatal(err)
	}
	// Disable cache and call again - should hit network.
	flagNoCache = true
	defer func() { flagNoCache = false }()
	if err := c.do("GET", "/apiv2/users/cache", nil, &out); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("GET", "/apiv2/users/cache")) != 2 {
		t.Errorf("expected 2 network calls (cache bypassed), got %d",
			len(st.requestsTo("GET", "/apiv2/users/cache")))
	}
}

func TestCache_QueryStringStripped(t *testing.T) {
	withCacheTempDir(t)
	if stripQuery("/apiv2/users/cache?foo=bar") != "/apiv2/users/cache" {
		t.Error("stripQuery did not remove ?foo=bar")
	}
	if stripQuery("/apiv2/x") != "/apiv2/x" {
		t.Error("stripQuery should pass through paths with no query")
	}
}

func TestCache_ClearWipesEverything(t *testing.T) {
	dir := withCacheTempDir(t)
	_ = cacheSet("/apiv2/users/cache", json.RawMessage(`{}`), 5*time.Minute)
	if err := cacheClear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected cache dir to be gone after Clear")
	}
}

func TestCache_ClearMissingNoop(t *testing.T) {
	if err := cacheClear(); err != nil {
		t.Errorf("expected Clear on missing to be no-op, got %v", err)
	}
}

func TestCmd_CacheList(t *testing.T) {
	withCacheTempDir(t)
	_ = cacheSet("/apiv2/users/cache", json.RawMessage(`[]`), 5*time.Minute)

	srv, _, _, buf := withTestEnv(t)
	_ = srv
	if err := runCmd(t, "cache", "list"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "/apiv2/users/cache") {
		t.Errorf("expected cached path in output: %s", buf.String())
	}
}

func TestCmd_CacheListEmpty(t *testing.T) {
	withCacheTempDir(t)
	srv, _, _, buf := withTestEnv(t)
	_ = srv
	if err := runCmd(t, "cache", "list"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(empty)") {
		t.Errorf("expected (empty), got: %s", buf.String())
	}
}

func TestCmd_CacheClear(t *testing.T) {
	dir := withCacheTempDir(t)
	_ = cacheSet("/apiv2/users/cache", json.RawMessage(`{}`), 5*time.Minute)
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	if err := runCmd(t, "cache", "clear"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected cache dir gone after `cache clear`")
	}
}

var _ = fmt.Sprintf
var _ = http.StatusOK
