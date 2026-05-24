package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	mrand "math/rand"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Request-policy defaults. INF-1314 specifies exponential backoff + jitter,
// capped retries, and a 60s default timeout per request.
const (
	defaultMaxRetries      = 3
	defaultBackoffBase     = 500 * time.Millisecond
	defaultBackoffMin      = 100 * time.Millisecond // floor so full-jitter never sleeps zero
	defaultBackoffMax      = 10 * time.Second
	defaultRetryAfterCap   = 60 * time.Second // an upstream `Retry-After: 86400` would otherwise park CI
	defaultResponseBodyCap = 4 * 1024 * 1024  // 4 MB
	defaultRequestTimeout  = 60 * time.Second
)

// userAgent builds the User-Agent string per INF-1314's spec:
//   <ctl>/<version> (<os>/<arch>)
func userAgent() string {
	v := version
	if v == "" {
		v = "dev"
	}
	return fmt.Sprintf("zeltapp/%s (%s/%s)", v, runtime.GOOS, runtime.GOARCH)
}

// newIdempotencyKey returns a random 32-hex-char idempotency token suitable
// for `Idempotency-Key` headers on POST/PUT/PATCH.
func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// extremely unlikely; fall back to a millis timestamp so we still send
		// something unique-ish rather than colliding.
		return fmt.Sprintf("ts-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// shouldRetry returns true for the network-failure / transient-status classes
// where a retry is appropriate. 429 and 5xx are explicit; 0 (no response)
// covers connection failures.
func shouldRetry(status int) bool {
	if status == 0 {
		return true
	}
	if status == 408 || status == 429 {
		return true
	}
	if status >= 500 && status < 600 {
		return true
	}
	return false
}

// backoffDuration computes exponential backoff with full jitter for attempt N
// (1-indexed). The result is bounded by [defaultBackoffMin, defaultBackoffMax]
// so a "full-jitter" draw of zero doesn't hammer the upstream during a 429
// storm.
func backoffDuration(attempt int) time.Duration {
	exp := float64(defaultBackoffBase) * math.Pow(2, float64(attempt-1))
	if exp > float64(defaultBackoffMax) {
		exp = float64(defaultBackoffMax)
	}
	d := time.Duration(mrand.Int63n(int64(exp) + 1))
	if d < defaultBackoffMin {
		d = defaultBackoffMin
	}
	return d
}

// parseRetryAfter returns the duration to wait per the Retry-After header.
// Accepts both delta-seconds and HTTP-date formats; capped at
// defaultRetryAfterCap so a misbehaving (or hostile) edge returning
// `Retry-After: 86400` cannot park the CLI for hours mid-CI.
func parseRetryAfter(h http.Header) time.Duration {
	val := h.Get("Retry-After")
	if val == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && secs >= 0 {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(val); err == nil {
		if until := time.Until(t); until > 0 {
			d = until
		}
	}
	if d > defaultRetryAfterCap {
		d = defaultRetryAfterCap
	}
	return d
}

// extractRequestID pulls a trace identifier from the response headers.
// Zelt uses multiple conventions across upstream/CloudFront; we accept any.
func extractRequestID(h http.Header) string {
	for _, k := range []string{"X-Request-Id", "X-Trace-Id", "X-Amz-Cf-Id", "Cf-Ray"} {
		if v := h.Get(k); v != "" {
			return v
		}
	}
	return ""
}

// redactSensitive strips credential-bearing headers from a cloned http.Header.
// Uses a pattern match rather than a closed allowlist so future header names
// (X-Auth-Token, Proxy-Authorization, X-Session, ...) don't silently leak in
// `-v` output.
func redactSensitive(h http.Header) http.Header {
	out := h.Clone()
	for k := range out {
		if isSensitiveHeader(k) {
			out.Set(k, "<redacted>")
		}
	}
	return out
}

func isSensitiveHeader(name string) bool {
	lc := strings.ToLower(name)
	for _, frag := range []string{"auth", "cookie", "token", "secret", "api-key", "apikey", "csrf"} {
		if strings.Contains(lc, frag) {
			return true
		}
	}
	return false
}
