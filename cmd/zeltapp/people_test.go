package main

import (
	"fmt"
	"strings"
	"testing"
)

// fixturePeopleCache mirrors the shape /apiv2/users/cache likely returns:
// an array of user summaries. Tests assume this layout.
var fixturePeopleCache = mustJSON([]map[string]any{
	{
		"userId": 1001, "firstName": "Alice", "lastName": "Smith",
		"displayName": "Alice Smith", "emailAddress": "alice@example.com",
		"jobPosition": "Engineer", "department": "Engineering", "site": "London",
		"accountStatus": "Active", "lifecycleStatus": "Employed",
	},
	{
		"userId": 1002, "firstName": "Bob", "lastName": "Jones",
		"displayName": "Bob Jones", "emailAddress": "bob@example.com",
		"jobPosition": "Designer", "department": "Product", "site": "Berlin",
		"accountStatus": "Active", "lifecycleStatus": "Employed",
	},
	{
		"userId": 1003, "firstName": "Carol", "lastName": "Lee",
		"displayName": "Carol Lee", "emailAddress": "carol@example.com",
		"jobPosition": "PM", "department": "Product", "site": "London",
		"accountStatus": "Active", "lifecycleStatus": "Terminated",
	},
	{
		"userId": 1004, "firstName": "Dan", "lastName": "Wong",
		"displayName": "Dan Wong", "emailAddress": "dan@example.com",
		"jobPosition": "Engineer", "department": "Engineering", "site": "Remote",
		"accountStatus": "Inactive", "lifecycleStatus": "Employed",
	},
})

func TestCmd_PeopleList_ActiveOnly(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmd(t, "people", "list"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Alice", "Bob"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in active list: %s", want, out)
		}
	}
	for _, notWant := range []string{"Carol", "Dan"} {
		if strings.Contains(out, notWant) {
			t.Errorf("did not expect %q (inactive/terminated): %s", notWant, out)
		}
	}
}

func TestCmd_PeopleList_All(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmd(t, "people", "list", "--all"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Alice", "Bob", "Carol", "Dan"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in --all list: %s", want, out)
		}
	}
}

func TestCmd_PeopleSearch(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmd(t, "people", "search", "engineer"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Alice (engineer/active) and Dan (engineer/inactive) both match jobPosition,
	// but inactive users are still included in search (search doesn't filter).
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected Alice (Engineer) in search: %s", out)
	}
	if strings.Contains(out, "Bob") || strings.Contains(out, "Carol") {
		t.Errorf("did not expect non-engineer matches: %s", out)
	}
}

func TestCmd_PeopleSearchByEmail(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmd(t, "people", "search", "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Bob") {
		t.Errorf("expected Bob in email search: %s", buf.String())
	}
}

func TestCmd_PeopleSearch_NoMatches(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmd(t, "people", "search", "noonehere"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no matches") {
		t.Errorf("expected '(no matches)', got: %s", buf.String())
	}
}

func TestCmd_PeopleGetByID(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	for _, p := range []string{"basic", "personal", "role", "work-contact"} {
		st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/%s", 1001, p), 200,
			mustJSON(map[string]any{"_section": p}))
	}
	if err := runCmd(t, "people", "get", "1001"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"basic", "personal", "role", "work-contact"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %q in get output: %s", want, buf.String())
		}
	}
}

func TestCmd_PeopleGetByEmail(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	for _, p := range []string{"basic", "personal", "role", "work-contact"} {
		st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/%s", 1002, p), 200, []byte(`{}`))
	}
	if err := runCmd(t, "people", "get", "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	// Verify cache was looked up to resolve email -> 1002
	if len(st.requestsTo("GET", "/apiv2/users/cache")) == 0 {
		t.Error("expected /users/cache to be hit for email resolution")
	}
}

func TestCmd_PeopleGetEmailNotFound(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	err := runCmd(t, "people", "get", "missing@example.com")
	if err == nil {
		t.Fatal("expected error for unknown email")
	}
}

func TestCmd_PeopleGetCustomSections(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/1001/equity", 200, []byte(`{}`))
	if err := runCmd(t, "people", "get", "1001", "--sections", "equity"); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("GET", "/apiv2/users/1001/equity")) != 1 {
		t.Error("expected /users/1001/equity to be hit")
	}
}

func TestPeopleCache_HandlesObjectWithUsersKey(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/users/cache", 200,
		mustJSON(map[string]any{"users": []map[string]any{
			{"userId": 1, "firstName": "X", "lastName": "Y", "emailAddress": "x@y"},
		}}))
	c, _ := newAuthedTestClient(t, srv)
	arr, _, err := fetchPeopleCache(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(arr) != 1 || arr[0].UserID != 1 {
		t.Errorf("unexpected result: %#v", arr)
	}
}

func TestPeopleCache_HandlesUnknownShape(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/users/cache", 200, []byte(`{"weird":"shape"}`))
	c, _ := newAuthedTestClient(t, srv)
	arr, raw, err := fetchPeopleCache(c)
	if err != nil {
		t.Fatal(err)
	}
	if arr != nil {
		t.Error("expected nil arr for unknown shape (fall back to raw)")
	}
	if !strings.Contains(string(raw), "weird") {
		t.Error("expected raw to contain original body")
	}
}
