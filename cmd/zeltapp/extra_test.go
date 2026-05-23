package main

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestTryRefresh_BodyToken(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("POST", "/apiv2/auth/refresh", 200,
		[]byte(`{"accessToken":"BODY_TOKEN","refreshToken":"BODY_REFRESH"}`))
	c, _ := newAuthedTestClient(t, srv)
	if err := c.tryRefresh(); err != nil {
		t.Fatal(err)
	}
	if c.session.Token != "BODY_TOKEN" {
		t.Errorf("expected BODY_TOKEN, got %s", c.session.Token)
	}
	if c.session.RefreshToken != "BODY_REFRESH" {
		t.Errorf("expected BODY_REFRESH, got %s", c.session.RefreshToken)
	}
}

func TestTryRefresh_AllCandidatesFail(t *testing.T) {
	srv, st := newTestServer(t)
	for _, p := range []string{"/apiv2/auth/refresh", "/apiv2/auth/refresh-token", "/apiv2/auth/token/refresh"} {
		st.jsonRoute("POST", p, 404, []byte(``))
	}
	c, _ := newAuthedTestClient(t, srv)
	if err := c.tryRefresh(); err == nil {
		t.Fatal("expected error when all refresh paths fail")
	}
}

func TestTryRefresh_FirstCandidateWins(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("POST", "/apiv2/auth/refresh", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		http.SetCookie(w, &http.Cookie{Name: "token", Value: "FIRST"})
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	})
	c, _ := newAuthedTestClient(t, srv)
	if err := c.tryRefresh(); err != nil {
		t.Fatal(err)
	}
	if c.session.Token != "FIRST" {
		t.Errorf("expected FIRST, got %s", c.session.Token)
	}
}

func TestWithClient_NoSession(t *testing.T) {
	srv, _ := newTestServer(t)
	store := newMemStore() // no session pre-populated
	prevHook := newClientHook
	newClientHook = func() (*client, error) {
		return newClient(withBaseURL(srv.URL), withStore(store))
	}
	defer func() { newClientHook = prevHook }()

	err := withClient(func(c *client) error { return nil })
	if err == nil {
		t.Fatal("expected error when no session")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("expected 'not logged in' in error, got: %v", err)
	}
}

func TestWithClient_FactoryError(t *testing.T) {
	prevHook := newClientHook
	newClientHook = func() (*client, error) { return nil, http.ErrAbortHandler }
	defer func() { newClientHook = prevHook }()

	err := withClient(func(c *client) error { return nil })
	if err == nil {
		t.Fatal("expected factory error to propagate")
	}
}

func TestLeaveBook_AbortsOnNo(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceVerifyOverlap)
	st.jsonRoute("POST", "/apiv2/absences/request-value-and-balance2", 201, fixtureRequestValueBalance)
	st.jsonRoute("POST", "/apiv2/absences/multiple", 201, fixtureBookSuccess)

	// Pipe "n\n" to stdin to simulate declining the prompt.
	r, w, _ := os.Pipe()
	w.WriteString("n\n")
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	err := runCmd(t, "leave", "book", "--policy", "512", "--start", "2030-06-01")
	if err == nil {
		t.Fatal("expected abort error")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected aborted in error, got: %v", err)
	}
	if len(st.requestsTo("POST", "/apiv2/absences/multiple")) != 0 {
		t.Error("book should not have been called after abort")
	}
}

func TestLeaveBook_ProceedsOnYes(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceVerifyOverlap)
	st.jsonRoute("POST", "/apiv2/absences/request-value-and-balance2", 201, fixtureRequestValueBalance)
	st.jsonRoute("POST", "/apiv2/absences/multiple", 201, fixtureBookSuccess)

	r, w, _ := os.Pipe()
	w.WriteString("y\n")
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	if err := runCmd(t, "leave", "book", "--policy", "512", "--start", "2030-06-01"); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("POST", "/apiv2/absences/multiple")) != 1 {
		t.Error("book should have been called after y")
	}
}
