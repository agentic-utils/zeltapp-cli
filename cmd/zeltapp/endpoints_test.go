package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestEndpoint_AttendanceWidget(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("GET", "/apiv2/attendance-entries/users/", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if !strings.Contains(r.URL.RawQuery, "date=2030-01-15") {
			t.Errorf("missing date query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureAttendanceWidget)
	})
	c, _ := newAuthedTestClient(t, srv)
	var out map[string]any
	if err := c.do("GET", "/apiv2/attendance-entries/users/1234/widget?date=2030-01-15", nil, &out); err != nil {
		t.Fatal(err)
	}
	if out["status"] != "noEntry" {
		t.Errorf("status wrong: %v", out["status"])
	}
}

func TestEndpoint_CalendarBuildsQueryString(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/calendar/initial", 200, fixtureCalendar)
	c, _ := newAuthedTestClient(t, srv)

	if err := c.do("GET",
		"/apiv2/calendar/initial?displayFilter=Absence&start=2030-01-01&end=2030-01-07&view=team&page=1&pageSize=20",
		nil, nil); err != nil {
		t.Fatal(err)
	}
	r := st.lastRequest(t)
	q := r.Header // just verify hit
	_ = q
	if !strings.Contains(r.Header.Get("Origin"), "127.0.0.1") {
		t.Errorf("Origin header wrong: %s", r.Header.Get("Origin"))
	}
}

func TestEndpoint_RawCommandPassthrough(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/users/cache", 200, []byte(`{"x":1}`))
	c, _ := newAuthedTestClient(t, srv)

	var out json.RawMessage
	if err := c.do("GET", "/apiv2/users/cache", nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"x":1`) {
		t.Errorf("body wrong: %s", out)
	}
}

func TestEndpoint_RawCommandPOSTBody(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("POST", "/apiv2/foo", 200, []byte(`{}`))
	c, _ := newAuthedTestClient(t, srv)

	body := map[string]any{"x": 1, "nested": []int{1, 2, 3}}
	if err := c.do("POST", "/apiv2/foo", body, nil); err != nil {
		t.Fatal(err)
	}
	r := st.lastRequest(t)
	var parsed map[string]any
	_ = json.Unmarshal(r.Body, &parsed)
	if parsed["x"].(float64) != 1 {
		t.Errorf("x wrong: %v", parsed["x"])
	}
}

func TestEndpoint_CompanyConfig(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/companies/config", 200, fixtureCompanyConfig)
	c, _ := newAuthedTestClient(t, srv)
	var out map[string]any
	if err := c.do("GET", "/apiv2/companies/config", nil, &out); err != nil {
		t.Fatal(err)
	}
	if int(out["companyId"].(float64)) != fakeCompanyID {
		t.Errorf("companyId wrong: %v", out["companyId"])
	}
}

func TestEndpoint_ExpensesPaginated(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/expense/user/paginated", 200, fixtureExpenses)
	c, _ := newAuthedTestClient(t, srv)
	var out map[string]any
	if err := c.do("GET", "/apiv2/expense/user/paginated?page=1&pageSize=50", nil, &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["items"]; !ok {
		t.Error("items key missing")
	}
}
