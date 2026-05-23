package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// withTestEnv sets up an httptest server, an in-memory store and rewires the
// global hooks so cobra commands use them. Returns a cleanup func.
func withTestEnv(t *testing.T) (*httptest.Server, *serverState, *memStore, *bytes.Buffer) {
	t.Helper()
	srv, state := newTestServer(t)
	store := newMemStore()
	// pre-authenticated session so withClient passes
	store.session = &session{
		Email: fakeEmail, UserID: fakeUserID, CompanyID: fakeCompanyID,
		DisplayName: fakeDisplayName, Token: fakeAccessToken, RefreshToken: fakeRefreshToken,
	}
	prevHook := newClientHook
	newClientHook = func() (*client, error) {
		return newClient(withBaseURL(srv.URL), withStore(store))
	}
	buf := &bytes.Buffer{}
	prevWriter := outWriter
	outWriter = buf
	prevJSON := flagJSON
	flagJSON = true // tests assert JSON; human-format tests set this back
	prevNoCache := flagNoCache
	flagNoCache = true // disable shared on-disk cache during tests
	t.Cleanup(func() {
		newClientHook = prevHook
		outWriter = prevWriter
		flagJSON = prevJSON
		flagNoCache = prevNoCache
	})
	return srv, state, store, buf
}

func runCmd(t *testing.T, args ...string) error {
	t.Helper()
	root := &cobra.Command{Use: "zeltapp", SilenceUsage: true, SilenceErrors: true}
	// Default to --json in tests so assertions can match raw JSON output.
	// Override per-test with runCmdHuman (or pass --json=false explicitly).
	root.PersistentFlags().BoolVar(&flagJSON, "json", true, "")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "")
	root.AddCommand(
		whoamiCmd(), meCmd(), peopleCmd(), leaveCmd(), attendanceCmd(), calendarCmd(),
		expensesCmd(), companyCmd(), reviewsCmd(), goalsCmd(), rawCmd(), cacheCmd(),
	)
	root.SetArgs(args)
	return root.Execute()
}

// runCmdHuman runs a command with human-readable output (no --json).
func runCmdHuman(t *testing.T, args ...string) error {
	t.Helper()
	root := &cobra.Command{Use: "zeltapp", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "")
	root.AddCommand(
		whoamiCmd(), meCmd(), peopleCmd(), leaveCmd(), attendanceCmd(), calendarCmd(),
		expensesCmd(), companyCmd(), reviewsCmd(), goalsCmd(), rawCmd(), cacheCmd(),
	)
	root.SetArgs(args)
	return root.Execute()
}

func TestCmd_Whoami(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/auth/me", 200, fixtureAuthMe)

	if err := runCmd(t, "whoami"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"displayName"`) {
		t.Errorf("expected JSON with displayName, got: %s", buf.String())
	}
}

func TestCmd_MeProfile(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	for _, p := range []string{"basic", "personal", "about", "missing-fields"} {
		st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/%s", fakeUserID, p), 200,
			mustJSON(map[string]any{"_path": p}))
	}
	if err := runCmd(t, "me", "profile"); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"basic", "personal", "about", "missing-fields"} {
		if _, ok := out[k]; !ok {
			t.Errorf("output missing key %q: %s", k, buf.String())
		}
	}
}

func TestCmd_MePension(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/employees/%d/pension", fakeUserID), 200,
		mustJSON(map[string]any{"scheme": "nest"}))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/employees/%d/pension/contributions", fakeUserID), 200,
		mustJSON([]map[string]any{{"month": "2030-01", "amount": 100}}))

	if err := runCmd(t, "me", "pension"); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["pension"]; !ok {
		t.Error("missing pension key")
	}
	if _, ok := out["contributions"]; !ok {
		t.Error("missing contributions key")
	}
}

func TestCmd_LeaveBalance(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/absences/users/leave-days", 200, fixtureLeaveDays)
	if err := runCmd(t, "leave", "balance"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Annual Leave") {
		t.Errorf("expected Annual Leave in output, got: %s", buf.String())
	}
}

func TestCmd_LeavePolicies(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/absence-policies/team/extended", 200, fixturePoliciesExtended)
	if err := runCmd(t, "leave", "policies"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_LeaveList(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/absences/manager", 200, []byte(`{"items":[],"total":0}`))
	if err := runCmd(t, "leave", "list"); err != nil {
		t.Fatal(err)
	}
	r := st.lastRequest(t)
	if !strings.Contains(r.Header.Get("Origin"), "127.0.0.1") {
		t.Error("Origin header missing or wrong")
	}
	_ = buf
}

func TestCmd_LeaveCheck(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceVerifyOverlap)
	st.jsonRoute("POST", "/apiv2/absences/request-value-and-balance2", 201, fixtureRequestValueBalance)
	if err := runCmd(t, "leave", "check", "--policy", "512", "--start", "2030-06-01"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_LeaveCheckRequiresFlags(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	err := runCmd(t, "leave", "check")
	if err == nil {
		t.Fatal("expected error when flags missing")
	}
}

func TestCmd_LeaveBookYesSkipsPrompt(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceVerifyOverlap)
	st.jsonRoute("POST", "/apiv2/absences/request-value-and-balance2", 201, fixtureRequestValueBalance)
	st.jsonRoute("POST", "/apiv2/absences/multiple", 201, fixtureBookSuccess)

	if err := runCmd(t, "leave", "book", "--policy", "512", "--start", "2030-06-01", "--yes"); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("POST", "/apiv2/absences/multiple")) != 1 {
		t.Error("book not called")
	}
}

func TestCmd_Attendance(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.route("GET", "/apiv2/attendance-entries/users/", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureAttendanceWidget)
	})
	if err := runCmd(t, "attendance", "today"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "noEntry") {
		t.Errorf("expected fixture content in output, got: %s", buf.String())
	}
}

func TestCmd_AttendanceTable(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.route("GET", "/apiv2/attendance-entries/users/", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rows":[]}`))
	})
	if err := runCmd(t, "attendance", "table"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_AttendanceSettings(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/attendance-settings", 200, []byte(`{}`))
	if err := runCmd(t, "attendance", "settings"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_CalendarTeamDefaultRange(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/calendar/initial", 200, fixtureCalendar)
	if err := runCmd(t, "calendar", "team"); err != nil {
		t.Fatal(err)
	}
	r := st.lastRequest(t)
	if !strings.Contains(r.Path, "calendar") {
		t.Errorf("wrong path: %s", r.Path)
	}
}

func TestCmd_CalendarTeamCustomRange(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/calendar/initial", 200, fixtureCalendar)
	if err := runCmd(t, "calendar", "team", "--start", "2030-01-01", "--end", "2030-01-07"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_Expenses(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/expense/user/paginated", 200, fixtureExpenses)
	if err := runCmd(t, "expenses", "list"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_ExpensesInvoices(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/contractor/invoice/users/%d", fakeUserID), 200, []byte(`[]`))
	if err := runCmd(t, "expenses", "invoices"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_CompanyAll(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	for _, path := range []string{
		"/apiv2/companies/config",
		"/apiv2/companies/general-settings",
		"/apiv2/companies/departments",
		"/apiv2/companies/sites",
		"/apiv2/job-positions",
		"/apiv2/company/forms",
		"/apiv2/company/fields/all-fields-profile",
		"/apiv2/reports/all/new",
		"/apiv2/apps",
		"/apiv2/apps/install",
	} {
		st.jsonRoute("GET", path, 200, []byte(`{}`))
	}
	for _, sub := range []string{"config", "general-settings", "departments", "sites",
		"job-positions", "forms", "fields", "reports", "apps", "apps-install"} {
		if err := runCmd(t, "company", sub); err != nil {
			t.Errorf("company %s failed: %v", sub, err)
		}
	}
}

func TestCmd_ReviewsAndGoals(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/review-cycle/ongoing/parents", 200, []byte(`[]`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/reviews/result/me/%d", fakeUserID), 200, []byte(`{}`))
	st.jsonRoute("GET", "/apiv2/goals", 200, []byte(`[]`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/goals/user/%d", fakeUserID), 200, []byte(`[]`))

	for _, args := range [][]string{
		{"reviews", "list"},
		{"reviews", "mine"},
		{"goals", "list"},
		{"goals", "mine"},
	} {
		if err := runCmd(t, args...); err != nil {
			t.Errorf("%v failed: %v", args, err)
		}
	}
}

func TestCmd_RawGet(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, []byte(`{"hit":true}`))
	if err := runCmd(t, "raw", "GET", "/apiv2/users/cache"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"hit"`) {
		t.Errorf("expected hit in output: %s", buf.String())
	}
}

func TestCmd_RawPostWithBody(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("POST", "/apiv2/foo", 200, []byte(`{}`))
	if err := runCmd(t, "raw", "POST", "/apiv2/foo", `{"x":1}`); err != nil {
		t.Fatal(err)
	}
	r := st.lastRequest(t)
	if !strings.Contains(string(r.Body), `"x":1`) {
		t.Errorf("body not propagated: %s", r.Body)
	}
}

func TestCmd_RawRejectsInvalidJSON(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	err := runCmd(t, "raw", "POST", "/apiv2/foo", `{not json`)
	if err == nil {
		t.Fatal("expected error on bad JSON")
	}
}

func TestCmd_RawAddsLeadingSlash(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/x", 200, []byte(`{}`))
	if err := runCmd(t, "raw", "GET", "apiv2/x"); err != nil {
		t.Fatal(err)
	}
}
