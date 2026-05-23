package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestMe_FetchManyAggregatesPaths(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/basic", fakeUserID), 200,
		mustJSON(map[string]any{"firstName": "Test", "lastName": "User"}))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/personal", fakeUserID), 200,
		mustJSON(map[string]any{"dob": "1990-01-01"}))
	c, _ := newAuthedTestClient(t, srv)

	out, err := fetchMany(c, "basic", "personal")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["basic"]; !ok {
		t.Error("basic missing")
	}
	if _, ok := out["personal"]; !ok {
		t.Error("personal missing")
	}
	var b map[string]any
	_ = json.Unmarshal(out["basic"], &b)
	if b["firstName"] != "Test" {
		t.Errorf("basic firstName wrong: %v", b["firstName"])
	}
}

func TestMe_AuthMeFixtureUnmarshal(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/auth/me", 200, fixtureAuthMe)
	c, _ := newAuthedTestClient(t, srv)

	var out map[string]any
	if err := c.do("GET", "/apiv2/auth/me", nil, &out); err != nil {
		t.Fatal(err)
	}
	user, _ := out["user"].(map[string]any)
	wantKeys := []string{"userId", "emailAddress", "firstName", "lastName", "displayName",
		"accountType", "company", "scopes2"}
	for _, k := range wantKeys {
		if _, ok := user[k]; !ok {
			t.Errorf("user missing key %q (matched against recorded schema)", k)
		}
	}
}

func TestMe_PathBuildsCorrectly(t *testing.T) {
	srv, st := newTestServer(t)
	want := fmt.Sprintf("/apiv2/users/%d/equity", fakeUserID)
	st.jsonRoute("GET", want, 200, []byte(`{"grants":[]}`))
	c, _ := newAuthedTestClient(t, srv)

	if _, err := fetchMany(c, "equity"); err != nil {
		t.Fatal(err)
	}
	r := st.lastRequest(t)
	if r.Path != want {
		t.Errorf("expected path %s, got %s", want, r.Path)
	}
}
