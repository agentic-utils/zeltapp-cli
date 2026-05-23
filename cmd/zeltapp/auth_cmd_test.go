package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// withDefaultStoreRedirect points the package-level defaultStore() helpers at a
// temp directory by setting XDG_CONFIG_HOME. Returns the temp dir.
func withDefaultStoreRedirect(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestCmd_Logout(t *testing.T) {
	dir := withDefaultStoreRedirect(t)
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	// pre-populate a session file on disk so the legacy loadSession / clearSession
	// helpers have something to remove.
	sessionFile := filepath.Join(dir, "zeltapp-cli", "session.json")
	if err := os.MkdirAll(filepath.Dir(sessionFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionFile, []byte(`{"email":"x@example.com"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// Run via the actual logoutCmd directly (so the test exercises the file path).
	cmd := logoutCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Error("session file should be gone after logout")
	}
}

func TestCompatShims(t *testing.T) {
	dir := withDefaultStoreRedirect(t)
	_ = dir

	// initially no session
	if _, err := loadSession(); err == nil {
		t.Error("expected error loading missing session")
	}

	s := &session{Email: "x", Token: "T"}
	if err := saveSession(s); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSession()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Email != "x" {
		t.Errorf("email lost: %v", loaded)
	}
	if got := sessionPath(); got == "" {
		t.Error("sessionPath empty")
	}
	if err := clearSession(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSession(); err == nil {
		t.Error("expected error after clearSession")
	}
}

func TestNewClientFromFlags(t *testing.T) {
	dir := withDefaultStoreRedirect(t)
	_ = dir
	c, err := newClientFromFlags(false)
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL wrong: %s", c.baseURL)
	}
	if c.http == nil {
		t.Error("http client nil")
	}
}

func TestWithHTTPOption(t *testing.T) {
	custom := &http.Client{}
	c, err := newClient(withHTTP(custom))
	if err != nil {
		t.Fatal(err)
	}
	if c.http != custom {
		t.Error("withHTTP did not install custom http client")
	}
}

func TestWithVerboseOption(t *testing.T) {
	c, err := newClient(withVerbose(true))
	if err != nil {
		t.Fatal(err)
	}
	if !c.verbose {
		t.Error("verbose flag not set")
	}
}

func TestApiErrorMessage(t *testing.T) {
	e := &apiError{Status: 500, Method: "GET", URL: "/x", Body: "boom"}
	msg := e.Error()
	for _, want := range []string{"500", "GET", "/x", "boom"} {
		if !contains(msg, want) {
			t.Errorf("expected error msg to contain %q, got %q", want, msg)
		}
	}
}

func TestRenderRawNonJSON(t *testing.T) {
	prev := outWriter
	prevJSON := flagJSON
	defer func() { outWriter = prev; flagJSON = prevJSON }()
	buf := &captureWriter{}
	outWriter = buf
	flagJSON = true
	if err := renderRaw([]byte(`not json`)); err != nil {
		t.Fatal(err)
	}
	if string(buf.data) != "not json" {
		t.Errorf("expected raw passthrough, got %q", buf.data)
	}
}

func TestRenderRawJSON(t *testing.T) {
	prev := outWriter
	prevJSON := flagJSON
	defer func() { outWriter = prev; flagJSON = prevJSON }()
	buf := &captureWriter{}
	outWriter = buf
	flagJSON = true
	if err := renderRaw([]byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if !contains(string(buf.data), `"a"`) {
		t.Errorf("expected pretty JSON, got %q", buf.data)
	}
}

type captureWriter struct{ data []byte }

func (c *captureWriter) Write(p []byte) (int, error) {
	c.data = append(c.data, p...)
	return len(p), nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ensure fmt import is used (avoid unused-import in case future edits drop it)
var _ = fmt.Sprintf
