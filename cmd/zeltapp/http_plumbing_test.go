package main

import (
	"errors"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUserAgent_Shape(t *testing.T) {
	prev := version
	version = "1.2.3"
	defer func() { version = prev }()
	want := "zeltapp/1.2.3 (" + runtime.GOOS + "/" + runtime.GOARCH + ")"
	if got := userAgent(); got != want {
		t.Errorf("userAgent() = %q, want %q", got, want)
	}
}

func TestIdempotencyKey_UniqueAndShape(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		k := newIdempotencyKey()
		if len(k) != 32 {
			t.Errorf("expected 32-char hex, got %d chars: %q", len(k), k)
		}
		if seen[k] {
			t.Fatalf("duplicate key %q after %d iterations", k, i)
		}
		seen[k] = true
	}
}

func TestShouldRetry_Classes(t *testing.T) {
	for _, code := range []int{0, 408, 429, 500, 502, 503, 504} {
		if !shouldRetry(code) {
			t.Errorf("expected retry for status %d", code)
		}
	}
	for _, code := range []int{200, 201, 301, 400, 401, 403, 404, 422} {
		if shouldRetry(code) {
			t.Errorf("did NOT expect retry for status %d", code)
		}
	}
}

func TestExtractRequestID_Sources(t *testing.T) {
	h := http.Header{}
	h.Set("X-Trace-Id", "trace-1")
	h.Set("X-Amz-Cf-Id", "cf-1")
	// X-Request-Id wins when present
	h2 := h.Clone()
	h2.Set("X-Request-Id", "req-1")
	if got := extractRequestID(h2); got != "req-1" {
		t.Errorf("X-Request-Id should win, got %q", got)
	}
	// X-Trace-Id is next
	if got := extractRequestID(h); got != "trace-1" {
		t.Errorf("X-Trace-Id fallback wrong: %q", got)
	}
	if got := extractRequestID(http.Header{}); got != "" {
		t.Errorf("empty headers should yield empty id, got %q", got)
	}
}

func TestRedactSensitive_StripsAuthHeaders(t *testing.T) {
	in := http.Header{}
	in.Set("Authorization", "Bearer abc123")
	in.Set("Cookie", "token=xyz")
	in.Set("X-Api-Key", "k")
	in.Set("Content-Type", "application/json")
	out := redactSensitive(in)
	for _, k := range []string{"Authorization", "Cookie", "X-Api-Key"} {
		if out.Get(k) != "<redacted>" {
			t.Errorf("%s not redacted: %q", k, out.Get(k))
		}
	}
	if out.Get("Content-Type") != "application/json" {
		t.Error("Content-Type should pass through")
	}
}

func TestParseRetryAfter_Seconds(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "5")
	d := parseRetryAfter(h)
	if d.Seconds() != 5 {
		t.Errorf("expected 5s, got %v", d)
	}
}

func TestApiError_IncludesRequestID(t *testing.T) {
	e := &apiError{Status: 500, Method: "GET", URL: "/x", Body: `{"message":"boom"}`, RequestID: "abc-123"}
	msg := e.Error()
	if !strings.Contains(msg, "request_id=abc-123") {
		t.Errorf("expected request_id in error: %s", msg)
	}
}

func TestApiError_ExitCodeMapping(t *testing.T) {
	cases := []struct {
		status int
		want   int
	}{
		{401, 3}, {403, 3}, {404, 4}, {429, 5}, {500, 6}, {503, 6}, {422, 1},
	}
	for _, c := range cases {
		e := &apiError{Status: c.status}
		if got := e.ExitCode(); got != c.want {
			t.Errorf("status %d -> exit %d, want %d", c.status, got, c.want)
		}
	}
}

func TestClient_RetriesOn503ThenSucceeds(t *testing.T) {
	srv, st := newTestServer(t)
	var calls atomic.Int32
	st.route("GET", "/apiv2/flaky", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	c, _ := newAuthedTestClient(t, srv)
	var out map[string]any
	if err := c.do("GET", "/apiv2/flaky", nil, &out); err != nil {
		t.Fatalf("expected eventual success after retries: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls (2 retries + success), got %d", calls.Load())
	}
}

func TestClient_RetriesExhaustReturnsAPIError(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("GET", "/apiv2/down", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.Header().Set("X-Request-Id", "req-down-1")
		w.WriteHeader(503)
		w.Write([]byte(`{"message":"down"}`))
	})
	c, _ := newAuthedTestClient(t, srv)
	err := c.do("GET", "/apiv2/down", nil, nil)
	var ae *apiError
	if !errors.As(err, &ae) {
		t.Fatalf("expected apiError, got %T %v", err, err)
	}
	if ae.RequestID != "req-down-1" {
		t.Errorf("expected request_id propagated, got %q", ae.RequestID)
	}
	if ae.ExitCode() != 6 {
		t.Errorf("expected 5xx exit code 6, got %d", ae.ExitCode())
	}
}

func TestClient_IdempotencyKeyOnWrite(t *testing.T) {
	srv, st := newTestServer(t)
	var key1, key2 string
	st.route("POST", "/apiv2/write", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if key1 == "" {
			key1 = r.Header.Get("Idempotency-Key")
		} else {
			key2 = r.Header.Get("Idempotency-Key")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	c, _ := newAuthedTestClient(t, srv)
	if err := c.do("POST", "/apiv2/write", map[string]any{"x": 1}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.do("POST", "/apiv2/write", map[string]any{"x": 1}, nil); err != nil {
		t.Fatal(err)
	}
	if key1 == "" || key2 == "" {
		t.Fatalf("Idempotency-Key not sent: %q / %q", key1, key2)
	}
	if key1 == key2 {
		t.Errorf("expected different keys across logical calls, got same %q", key1)
	}
}

func TestClient_GETSendsNoIdempotencyKey(t *testing.T) {
	srv, st := newTestServer(t)
	var sawKey bool
	st.route("GET", "/apiv2/read", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.Header.Get("Idempotency-Key") != "" {
			sawKey = true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	c, _ := newAuthedTestClient(t, srv)
	if err := c.do("GET", "/apiv2/read", nil, nil); err != nil {
		t.Fatal(err)
	}
	if sawKey {
		t.Error("GET should not send Idempotency-Key")
	}
}
