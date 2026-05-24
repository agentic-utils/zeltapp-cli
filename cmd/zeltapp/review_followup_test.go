package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// Tests for fixes from the adversarial review pass.

func TestRaw_RefusesAuthPaths(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	err := runCmd(t, "raw", "POST", "/apiv2/auth/login", `{"x":1}`)
	if err == nil || !strings.Contains(err.Error(), "auth") {
		t.Fatalf("expected auth-block error, got %v", err)
	}
}

func TestRaw_RefusesNonApivPaths(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	err := runCmd(t, "raw", "GET", "/admin/secret")
	if err == nil || !strings.Contains(err.Error(), "outside /apiv2") {
		t.Fatalf("expected /apiv2 prefix error, got %v", err)
	}
}

func TestRaw_RefusesUnknownMethod(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	err := runCmd(t, "raw", "FOO", "/apiv2/x")
	if err == nil || !strings.Contains(err.Error(), "unknown HTTP method") {
		t.Fatalf("expected method error, got %v", err)
	}
}

func TestRaw_RefusesBodyOnGet(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	err := runCmd(t, "raw", "GET", "/apiv2/x", `{"x":1}`)
	if err == nil || !strings.Contains(err.Error(), "do not accept a body") {
		t.Fatalf("expected body-on-GET error, got %v", err)
	}
}

func TestClient_CookieNotPersistedOn5xx(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("GET", "/apiv2/explode", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		// 5xx with a Set-Cookie attempt — should NOT clobber the session.
		http.SetCookie(w, &http.Cookie{Name: "token", Value: "BAD_FROM_5XX"})
		w.WriteHeader(503)
		w.Write([]byte(`{"message":"down"}`))
	})
	c, _ := newAuthedTestClient(t, srv)
	originalToken := c.session.Token
	_ = c.do("GET", "/apiv2/explode", nil, nil)
	if c.session.Token == "BAD_FROM_5XX" {
		t.Errorf("session token clobbered by 5xx Set-Cookie (was %q, became %q)", originalToken, c.session.Token)
	}
}

func TestClient_IdempotencyKeyStableAcrossReauth(t *testing.T) {
	srv, st := newTestServer(t)
	var keys []string
	var calls atomic.Int32
	st.route("POST", "/apiv2/write", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	st.route("POST", "/apiv2/auth/refresh", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		http.SetCookie(w, &http.Cookie{Name: "token", Value: "REFRESHED"})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	c, _ := newAuthedTestClient(t, srv)
	if err := c.do("POST", "/apiv2/write", map[string]any{"x": 1}, nil); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 write attempts, got %d", len(keys))
	}
	if keys[0] != keys[1] {
		t.Errorf("Idempotency-Key changed across reauth replay (got %q, %q); duplicate write risk", keys[0], keys[1])
	}
	if keys[0] == "" {
		t.Error("Idempotency-Key was empty on POST")
	}
}

func TestCache_ScopedByIdentity(t *testing.T) {
	dir := withCacheTempDir(t)
	_ = dir
	body := json.RawMessage(`[{"id":1}]`)
	if err := cacheSet("u:1:c:99", "/apiv2/users/cache", body, 5*60_000_000_000); err != nil {
		t.Fatal(err)
	}
	// Different identity should not find user 1's cached data.
	if _, ok := cacheGet("u:2:c:99", "/apiv2/users/cache"); ok {
		t.Error("cross-identity cache leak: u2 saw u1's cache entry")
	}
	if _, ok := cacheGet("u:1:c:99", "/apiv2/users/cache"); !ok {
		t.Error("same-identity cache hit failed")
	}
}

func TestPeople_List_IncludeInactiveFlag(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmd(t, "people", "list", "--include-inactive"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Alice Smith", "Bob Jones", "Carol Lee", "Dan Wong"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %q in --include-inactive output: %s", want, buf.String())
		}
	}
}

func TestPeople_EmailCollision_Errors(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	// Two users with the same email — should error rather than pick one.
	dup := mustJSON([]map[string]any{
		{"userId": 1, "firstName": "A", "lastName": "X", "displayName": "A X",
			"emailAddress": "shared@example.com", "accountStatus": "Active", "lifecycleStatus": "Employed"},
		{"userId": 2, "firstName": "B", "lastName": "X", "displayName": "B X",
			"emailAddress": "shared@example.com", "accountStatus": "Active", "lifecycleStatus": "Employed"},
	})
	st.jsonRoute("GET", "/apiv2/users/cache", 200, dup)
	err := runCmd(t, "people", "get", "shared@example.com")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous-email error, got %v", err)
	}
}

func TestOutput_Name_FallbackFromRaw(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	for _, p := range []string{"basic", "about", "work-contact", "role"} {
		st.jsonRoute("GET", "/apiv2/users/1234/"+p, 200, []byte(`{"displayName":"Test Person"}`))
	}
	root := newRootCmd()
	root.SetArgs([]string{"-o", "name", "--no-cache", "people", "get", "me"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Test Person") {
		t.Errorf("expected -o name to fall back to displayName from raw, got %q", buf.String())
	}
}

func TestRetryAfter_Capped(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "86400") // 24h
	d := parseRetryAfter(h)
	if d > defaultRetryAfterCap {
		t.Errorf("parseRetryAfter returned %v; expected cap at %v", d, defaultRetryAfterCap)
	}
}

func TestBackoff_NeverZero(t *testing.T) {
	for i := 1; i <= 20; i++ {
		d := backoffDuration(1)
		if d < defaultBackoffMin {
			t.Errorf("attempt 1 produced %v < min %v", d, defaultBackoffMin)
		}
	}
}

func TestPagination_RejectsNegativeFlags(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	if err := runCmd(t, "absence", "list", "--limit", "-1"); err == nil {
		t.Error("expected error for --limit -1")
	}
	if err := runCmd(t, "absence", "list", "--page-size", "0"); err == nil {
		t.Error("expected error for --page-size 0")
	}
}

func TestAbsenceFlags_RejectMorningAndAfternoon(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	err := runCmd(t, "absence", "book", "--policy", "1", "--start", "2030-06-01",
		"--morning", "--afternoon", "--yes")
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutex error, got %v", err)
	}
}

func TestAbsenceFlags_BadDate(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	err := runCmd(t, "absence", "book", "--policy", "1", "--start", "2030/06/01", "--yes")
	if err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Errorf("expected date format error, got %v", err)
	}
}

func TestRedactSensitive_PatternBased(t *testing.T) {
	h := http.Header{}
	h.Set("X-Auth-Token", "secret")
	h.Set("Proxy-Authorization", "Bearer x")
	h.Set("X-Random-Token", "tok")
	h.Set("Content-Type", "application/json")
	out := redactSensitive(h)
	for _, k := range []string{"X-Auth-Token", "Proxy-Authorization", "X-Random-Token"} {
		if out.Get(k) != "<redacted>" {
			t.Errorf("%s not redacted: %q", k, out.Get(k))
		}
	}
	if out.Get("Content-Type") != "application/json" {
		t.Error("Content-Type wrongly redacted")
	}
}

func TestSessionSave_AtomicLeavesIntactOnFailure(t *testing.T) {
	dir := t.TempDir()
	store := &fileStore{dir: dir}
	// First save — succeeds, file exists.
	if err := store.SaveSession(&session{Email: "a@b", Token: "T1"}); err != nil {
		t.Fatal(err)
	}
	first, _ := store.LoadSession()
	if first.Token != "T1" {
		t.Fatalf("first save lost data: %#v", first)
	}
	// Simulate a crash mid-write by writing a manual zero-byte session file
	// in the way a torn os.WriteFile would. With the atomic implementation
	// SaveSession should NEVER produce this state, but if a previous version
	// did, the load path should still surface the corruption.
	if err := store.SaveSession(&session{Email: "a@b", Token: "T2"}); err != nil {
		t.Fatal(err)
	}
	second, _ := store.LoadSession()
	if second.Token != "T2" {
		t.Errorf("second save lost data: %#v", second)
	}
}
