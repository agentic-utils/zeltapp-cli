package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMain disables the on-disk cache for all tests to avoid cross-test bleed.
func TestMain(m *testing.M) {
	flagNoCache = true
	os.Exit(m.Run())
}

// withTestEnv sets up an httptest server, an authenticated in-memory store, and
// rewires global hooks so cobra commands use them. Returns the server, the
// route registry, the store, and a buffer capturing outWriter.
func withTestEnv(t *testing.T) (*httptest.Server, *serverState, *memStore, *strings.Builder) {
	t.Helper()
	srv, state := newTestServer(t)
	store := newMemStore()
	store.session = &session{
		Email: fakeEmail, UserID: fakeUserID, CompanyID: fakeCompanyID,
		DisplayName: fakeDisplayName, Token: fakeAccessToken, RefreshToken: fakeRefreshToken,
	}
	prevHook := newClientHook
	newClientHook = func() (*client, error) {
		return newClient(withBaseURL(srv.URL), withStore(store))
	}
	buf := &strings.Builder{}
	prevWriter := outWriter
	outWriter = buf
	prevOutput := flagOutput
	flagOutput = outputJSON // tests assert JSON; switch with runCmdHuman for human-format tests
	prevNoCache := flagNoCache
	flagNoCache = true
	t.Cleanup(func() {
		newClientHook = prevHook
		outWriter = prevWriter
		flagOutput = prevOutput
		flagNoCache = prevNoCache
	})
	return srv, state, store, buf
}

// runCmd builds a fresh root command and runs it with the given args. The
// "-o json --no-cache" prefix is injected so tests can decode the output as
// JSON and don't accidentally read the real on-disk cache.
func runCmd(t *testing.T, args ...string) error {
	t.Helper()
	fullArgs := append([]string{"-o", "json", "--no-cache"}, args...)
	root := newRootCmd()
	root.SetArgs(fullArgs)
	return root.Execute()
}

// runCmdHuman runs a command in default (table/human) mode.
func runCmdHuman(t *testing.T, args ...string) error {
	t.Helper()
	fullArgs := append([]string{"--no-cache"}, args...)
	root := newRootCmd()
	root.SetArgs(fullArgs)
	return root.Execute()
}

// memStore is an in-memory store used by tests. Avoids touching the real
// filesystem and the macOS Keychain.
type memStore struct {
	mu        sync.Mutex
	session   *session
	passwords map[string]string
}

func newMemStore() *memStore { return &memStore{passwords: map[string]string{}} }

func (m *memStore) LoadSession() (*session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session == nil {
		return nil, errors.New("not found")
	}
	s := *m.session
	return &s, nil
}
func (m *memStore) SaveSession(s *session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *s
	cp.SavedAt = time.Now()
	m.session = &cp
	return nil
}
func (m *memStore) ClearSession() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session = nil
	return nil
}
func (m *memStore) GetPassword(email string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.passwords[email]; ok {
		return p, nil
	}
	return "", errors.New("not found")
}
func (m *memStore) SetPassword(email, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.passwords[email] = password
	return nil
}
func (m *memStore) DeletePassword(email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.passwords, email)
	return nil
}

// Recorded but anonymised JSON shapes - based on real Zelt responses
// (names, IDs, tokens, JWTs all replaced with synthetic values).

const (
	fakeEmail        = "test.user@example.com"
	fakeUserID       = 1234
	fakeCompanyID    = 99
	fakeDisplayName  = "Test User"
	fakeAccessToken  = "FAKE.ACCESS.TOKEN"
	fakeRefreshToken = "FAKE.REFRESH.TOKEN"
	fakePassword     = "correct-horse-battery-staple"
)

// fixtureAuthMe matches the shape of GET /apiv2/auth/me with anonymised fields.
var fixtureAuthMe = mustJSON(map[string]any{
	"user": map[string]any{
		"userId":       fakeUserID,
		"emailAddress": fakeEmail,
		"firstName":    "Test",
		"lastName":     "User",
		"displayName":  fakeDisplayName,
		"accountType": map[string]any{
			"Manager": true, "ProfileOwner": true, "OtherProfiles": true,
		},
		"accountStatus": "Active",
		"company": map[string]any{
			"companyId": fakeCompanyID, "name": "Example Co", "slug": "example-co",
		},
		"mfaType":      "email",
		"language":     "en",
		"currency":     "GBP",
		"contractType": "Payrolled",
		"scopes2":      []map[string]string{{"scope": "user"}, {"scope": "absence"}},
	},
	"isSuperAdmin":      false,
	"hasUnpaidInvoices": false,
	"publicURL":         "https://go.zelt.app/files",
})

var fixtureLoginStep1 = mustJSON(map[string]any{"mfaType": "email"})

var fixtureLoginStep2 = mustJSON(map[string]any{
	"accessToken": fakeAccessToken,
	"userId":      fakeUserID,
	"companyId":   fakeCompanyID,
	"displayName": fakeDisplayName,
})

// fixtureAbsenceVerifyOverlap mirrors /apiv2/absences/verify-overlap.
var fixtureAbsenceVerifyOverlap = mustJSON(map[string]any{
	"isOverlapping":      false,
	"absences":           []any{},
	"events":             []any{},
	"preventOwnOverlaps": true,
})

var fixtureAbsenceOverlap = mustJSON(map[string]any{
	"isOverlapping": true,
	"absences": []map[string]any{
		{
			"absenceId": 10001, "userId": 5678,
			"start": "2030-01-01", "end": "2030-01-05",
			"user": map[string]any{"userId": 5678, "firstName": "Jane", "lastName": "Doe"},
		},
	},
	"events":             []any{},
	"preventOwnOverlaps": true,
})

var fixtureRequestValueBalance = mustJSON(map[string]any{
	"requestValue":     1.0,
	"remainingBalance": 22.5,
	"unit":             "day",
	"accruedByDate":    nil,
	"usedSoFar":        2.5,
	"accrualDate":      nil,
})

var fixtureBookSuccess = mustJSON(map[string]any{
	"success":             true,
	"method":              "direct",
	"noOfCreatedAbsences": 1,
	"skippedUsers":        []any{},
})

var fixtureLeaveDays = mustJSON(map[string]any{
	"policies": []map[string]any{
		{"policyId": 512, "name": "Annual Leave", "remaining": 22.5, "unit": "day"},
	},
})

var fixturePoliciesExtended = mustJSON([]map[string]any{
	{"id": 512, "name": "Annual Leave", "type": "annual"},
	{"id": 513, "name": "Sick Leave", "type": "sick"},
})

var fixtureAttendanceWidget = mustJSON(map[string]any{
	"date":   "2030-01-15",
	"status": "noEntry",
	"hours":  0,
})

var fixtureCalendar = mustJSON(map[string]any{
	"users":  []any{},
	"days":   []any{},
	"totals": map[string]int{"absences": 0, "events": 0},
})

var fixtureExpenses = mustJSON(map[string]any{
	"items": []any{},
	"total": 0,
})

var fixtureCompanyConfig = mustJSON(map[string]any{
	"companyId": fakeCompanyID,
	"name":      "Example Co",
})

// mustJSON converts a Go value to a JSON byte slice; panics on failure (tests only).
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// recordedRequest is what handlers stash in serverState for assertions.
type recordedRequest struct {
	Method string
	Path   string
	Body   []byte
	Header http.Header
}

type routeHandler func(w http.ResponseWriter, r *http.Request, body []byte)

type serverState struct {
	mu       sync.Mutex
	requests []recordedRequest
	routes   map[string]routeHandler // key: METHOD PATH (path without query)
	notFound routeHandler
}

func newServerState() *serverState {
	return &serverState{routes: map[string]routeHandler{}}
}

// route registers a handler for METHOD PATH. PATH is the prefix the request must start with
// (so query strings are ignored).
func (s *serverState) route(method, path string, h routeHandler) {
	s.routes[method+" "+path] = h
}

// jsonRoute is a convenience: returns the given body with a status.
func (s *serverState) jsonRoute(method, path string, status int, body []byte) {
	s.route(method, path, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
	})
}

func (s *serverState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body := readAll(r)
	s.mu.Lock()
	s.requests = append(s.requests, recordedRequest{
		Method: r.Method, Path: r.URL.Path, Body: body, Header: r.Header.Clone(),
	})
	s.mu.Unlock()
	key := r.Method + " " + r.URL.Path
	if h, ok := s.routes[key]; ok {
		h(w, r, body)
		return
	}
	// Try prefix matches (for paths with trailing IDs)
	for k, h := range s.routes {
		parts := strings.SplitN(k, " ", 2)
		if parts[0] == r.Method && strings.HasPrefix(r.URL.Path, parts[1]) {
			h(w, r, body)
			return
		}
	}
	if s.notFound != nil {
		s.notFound(w, r, body)
		return
	}
	http.Error(w, fmt.Sprintf("no route for %s %s", r.Method, r.URL.Path), 404)
}

func (s *serverState) lastRequest(t *testing.T) recordedRequest {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		t.Fatal("no requests recorded")
	}
	return s.requests[len(s.requests)-1]
}

func (s *serverState) requestsTo(method, path string) []recordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []recordedRequest
	for _, r := range s.requests {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func newTestServer(t *testing.T) (*httptest.Server, *serverState) {
	t.Helper()
	state := newServerState()
	srv := httptest.NewServer(state)
	t.Cleanup(srv.Close)
	return srv, state
}

// newTestClient builds a client pointed at the test server with a memStore.
func newTestClient(t *testing.T, srv *httptest.Server) (*client, *memStore) {
	t.Helper()
	store := newMemStore()
	c, err := newClient(
		withBaseURL(srv.URL),
		withStore(store),
		withNow(func() time.Time { return time.Date(2030, 1, 15, 9, 0, 0, 0, time.UTC) }),
	)
	if err != nil {
		t.Fatal(err)
	}
	return c, store
}

// newAuthedTestClient pre-populates a session so requireSession() passes.
func newAuthedTestClient(t *testing.T, srv *httptest.Server) (*client, *memStore) {
	t.Helper()
	c, store := newTestClient(t, srv)
	store.session = &session{
		Email: fakeEmail, UserID: fakeUserID, CompanyID: fakeCompanyID,
		DisplayName: fakeDisplayName, Token: fakeAccessToken, RefreshToken: fakeRefreshToken,
	}
	// reload so the client picks it up
	c.session, _ = store.LoadSession()
	c.setCookies(c.session.Token, c.session.RefreshToken)
	return c, store
}

func readAll(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	b, _ := readFull(r)
	return b
}

func readFull(r *http.Request) ([]byte, error) {
	var sink strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			sink.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return []byte(sink.String()), nil
}
