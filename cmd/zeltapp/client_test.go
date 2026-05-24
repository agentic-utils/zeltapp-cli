package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestClient_DoHappyPath(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/auth/me", 200, fixtureAuthMe)
	c, _ := newAuthedTestClient(t, srv)

	var out map[string]any
	if err := c.do("GET", "/apiv2/auth/me", nil, &out); err != nil {
		t.Fatal(err)
	}
	user, ok := out["user"].(map[string]any)
	if !ok {
		t.Fatalf("missing user key: %#v", out)
	}
	if user["displayName"] != fakeDisplayName {
		t.Errorf("expected displayName=%s, got %v", fakeDisplayName, user["displayName"])
	}

	req := st.lastRequest(t)
	if req.Header.Get("X-Timezone") == "" {
		t.Error("X-Timezone header missing")
	}
	if req.Header.Get("X-Now-String") != "2030-01-15 09:00:00" {
		t.Errorf("X-Now-String wrong: %s", req.Header.Get("X-Now-String"))
	}
	if req.Header.Get("User-Agent") == "" {
		t.Error("User-Agent missing")
	}
}

func TestClient_DoSendsJSONBody(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("POST", "/apiv2/test", 201, []byte(`{"ok":true}`))
	c, _ := newAuthedTestClient(t, srv)

	type req struct {
		X int `json:"x"`
	}
	if err := c.do("POST", "/apiv2/test", req{X: 42}, nil); err != nil {
		t.Fatal(err)
	}
	r := st.lastRequest(t)
	if r.Header.Get("Content-Type") != "application/json" {
		t.Error("Content-Type not set on POST")
	}
	var parsed map[string]any
	if err := json.Unmarshal(r.Body, &parsed); err != nil {
		t.Fatal(err)
	}
	if v, _ := parsed["x"].(float64); v != 42 {
		t.Errorf("body x=42 expected, got %v", parsed["x"])
	}
}

func TestClient_DoReturnsAPIError(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/boom", 422, []byte(`{"error":"bad"}`))
	c, _ := newAuthedTestClient(t, srv)

	err := c.do("GET", "/apiv2/boom", nil, nil)
	var ae *apiError
	if !errors.As(err, &ae) {
		t.Fatalf("expected apiError, got %T %v", err, err)
	}
	if ae.Status != 422 {
		t.Errorf("status=%d expected 422", ae.Status)
	}
	if !strings.Contains(ae.Body, "bad") {
		t.Errorf("body did not propagate: %s", ae.Body)
	}
}

func TestClient_DoErrorWhenNoSession(t *testing.T) {
	srv, _ := newTestServer(t)
	c, _ := newTestClient(t, srv)
	if err := c.requireSession(); err == nil {
		t.Error("expected requireSession to fail with no session")
	}
}

func TestClient_RotatesCookiesOnResponse(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("GET", "/apiv2/ping", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		http.SetCookie(w, &http.Cookie{Name: "token", Value: "ROTATED"})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	c, store := newAuthedTestClient(t, srv)

	if err := c.do("GET", "/apiv2/ping", nil, nil); err != nil {
		t.Fatal(err)
	}
	if c.session.Token != "ROTATED" {
		t.Errorf("token not rotated in client: %s", c.session.Token)
	}
	if store.session == nil || store.session.Token != "ROTATED" {
		t.Errorf("token not rotated in store: %#v", store.session)
	}
}

func TestClient_401TriggersRefreshThenRetry(t *testing.T) {
	srv, st := newTestServer(t)
	// /apiv2/me: 401 first time, 200 after refresh
	calls := 0
	st.route("GET", "/apiv2/auth/me", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		calls++
		if calls == 1 {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureAuthMe)
	})
	st.route("POST", "/apiv2/auth/refresh", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		http.SetCookie(w, &http.Cookie{Name: "token", Value: "REFRESHED"})
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	})
	c, store := newAuthedTestClient(t, srv)

	var out map[string]any
	if err := c.do("GET", "/apiv2/auth/me", nil, &out); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("expected /auth/me to be hit twice (401 then 200), got %d", calls)
	}
	if c.session.Token != "REFRESHED" {
		t.Errorf("token not refreshed: %s", c.session.Token)
	}
	if store.session.Token != "REFRESHED" {
		t.Error("refreshed token not saved to store")
	}
}

func TestClient_401AndRefreshFailsButPasswordSucceeds(t *testing.T) {
	srv, st := newTestServer(t)
	// First call to /apiv2/data: 401. After re-login: 200.
	dataCalls := 0
	st.route("GET", "/apiv2/data", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		dataCalls++
		if dataCalls == 1 {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	// refresh always fails
	st.jsonRoute("POST", "/apiv2/auth/refresh", 401, []byte(`{"error":"expired"}`))
	st.jsonRoute("POST", "/apiv2/auth/refresh-token", 404, []byte(``))
	st.jsonRoute("POST", "/apiv2/auth/token/refresh", 404, []byte(``))
	// login step1 / step2
	loginCalls := 0
	st.route("POST", "/apiv2/auth/login", func(w http.ResponseWriter, r *http.Request, body []byte) {
		loginCalls++
		w.Header().Set("Content-Type", "application/json")
		if loginCalls == 1 {
			w.Write(fixtureLoginStep1)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "token", Value: "NEW_AFTER_REAUTH"})
		w.Write(fixtureLoginStep2)
	})

	c, store := newAuthedTestClient(t, srv)
	store.SetPassword(fakeEmail, fakePassword)

	// Inject a deterministic MFA prompter via direct method call ‒ but reauth uses
	// the package-level promptMFA. We need a way to override. Use a hook.
	prevPrompt := mfaPromptHook
	mfaPromptHook = func(method string) (string, error) { return "123456", nil }
	defer func() { mfaPromptHook = prevPrompt }()

	var out map[string]any
	if err := c.do("GET", "/apiv2/data", nil, &out); err != nil {
		t.Fatal(err)
	}
	if dataCalls != 2 {
		t.Errorf("expected /data to retry once after re-login, got %d calls", dataCalls)
	}
	if loginCalls != 2 {
		t.Errorf("expected 2 login calls (step1+step2), got %d", loginCalls)
	}
	if c.session.Token != "NEW_AFTER_REAUTH" {
		t.Errorf("expected new token after re-login, got %s", c.session.Token)
	}
}

func TestClient_401WithoutPasswordReturnsUnauthorized(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/data", 401, []byte(``))
	st.jsonRoute("POST", "/apiv2/auth/refresh", 401, []byte(``))
	st.jsonRoute("POST", "/apiv2/auth/refresh-token", 404, []byte(``))
	st.jsonRoute("POST", "/apiv2/auth/token/refresh", 404, []byte(``))
	c, _ := newAuthedTestClient(t, srv)
	// no stored password

	err := c.do("GET", "/apiv2/data", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("expected errUnauthorized, got %v", err)
	}
}

func TestPasswordLogin_Roundtrip(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("POST", "/apiv2/auth/login", func() routeHandler {
		i := 0
		return func(w http.ResponseWriter, r *http.Request, body []byte) {
			i++
			w.Header().Set("Content-Type", "application/json")
			if i == 1 {
				var parsed map[string]any
				json.Unmarshal(body, &parsed)
				if parsed["mfaCode"] != nil {
					t.Error("step 1 should not have mfaCode")
				}
				w.Write(fixtureLoginStep1)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "token", Value: "STEP2_TOKEN"})
			http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: "STEP2_REFRESH"})
			w.Write(fixtureLoginStep2)
		}
	}())
	c, store := newTestClient(t, srv)

	mfaCalled := false
	prompt := func(method string) (string, error) {
		mfaCalled = true
		if method != "email" {
			t.Errorf("expected mfaType=email, got %s", method)
		}
		return "424242", nil
	}
	if err := c.passwordLogin(fakeEmail, fakePassword, prompt); err != nil {
		t.Fatal(err)
	}
	if !mfaCalled {
		t.Error("MFA prompt should have been invoked")
	}
	if c.session.Email != fakeEmail {
		t.Errorf("session email wrong: %s", c.session.Email)
	}
	if c.session.UserID != fakeUserID {
		t.Errorf("session userId wrong: %d", c.session.UserID)
	}
	if c.session.Token != "STEP2_TOKEN" {
		t.Errorf("session token wrong: %s", c.session.Token)
	}
	if c.session.RefreshToken != "STEP2_REFRESH" {
		t.Errorf("session refresh wrong: %s", c.session.RefreshToken)
	}
	if store.session == nil {
		t.Fatal("session not persisted to store")
	}
}

func TestPasswordLogin_NoMFA(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("POST", "/apiv2/auth/login", func() routeHandler {
		i := 0
		return func(w http.ResponseWriter, r *http.Request, body []byte) {
			i++
			w.Header().Set("Content-Type", "application/json")
			if i == 1 {
				w.Write([]byte(`{"mfaType":""}`))
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "token", Value: "T"})
			w.Write(fixtureLoginStep2)
		}
	}())
	c, _ := newTestClient(t, srv)
	promptCalls := 0
	prompt := func(method string) (string, error) {
		promptCalls++
		return "", nil
	}
	if err := c.passwordLogin(fakeEmail, fakePassword, prompt); err != nil {
		t.Fatal(err)
	}
	if promptCalls != 0 {
		t.Errorf("MFA prompt should NOT have been invoked, got %d calls", promptCalls)
	}
}

func TestErrBody_Truncates(t *testing.T) {
	long := strings.Repeat("x", 1000)
	ae := &apiError{Status: 500, Method: "GET", URL: "/x", Body: long}
	out := errBody(ae)
	if len(out) > 510 {
		t.Errorf("expected truncated, got len=%d", len(out))
	}
	if !strings.HasSuffix(out, "...") {
		t.Errorf("expected ... suffix, got %q", out[len(out)-5:])
	}
}

func TestErrBody_PassesThroughPlainErrors(t *testing.T) {
	if got := errBody(errors.New("plain")); got != "plain" {
		t.Errorf("expected plain, got %q", got)
	}
}
