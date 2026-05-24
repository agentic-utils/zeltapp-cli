package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// ---------- people ----------

func TestPeople_List_Active(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmd(t, "people", "list"); err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &out); err != nil {
		t.Fatalf("output not JSON: %s", buf.String())
	}
	names := map[string]bool{}
	for _, r := range out {
		names[r["name"].(string)] = true
	}
	if !names["Alice Smith"] || !names["Bob Jones"] {
		t.Errorf("expected Alice + Bob in active list: %v", names)
	}
	if names["Carol Lee"] || names["Dan Wong"] {
		t.Errorf("inactive users leaked in: %v", names)
	}
}

func TestPeople_List_Alias(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	// `users ls` should work as alias for `people list`
	if err := runCmd(t, "users", "ls"); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("GET", "/apiv2/users/cache")) != 1 {
		t.Error("alias `users ls` should hit users/cache")
	}
}

func TestPeople_List_LabelSelector(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmd(t, "people", "list", "-l", "department=Product"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Alice Smith") {
		t.Errorf("Engineering filtered out by selector: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "Bob Jones") {
		t.Errorf("Product/Bob should match selector: %s", buf.String())
	}
}

func TestPeople_Get_Self(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	for _, p := range []string{"basic", "about", "work-contact", "role", "personal", "address", "family", "emergency-contact"} {
		st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/%s", fakeUserID, p), 200, []byte(`{}`))
	}
	if err := runCmd(t, "people", "get", "me"); err != nil {
		t.Fatal(err)
	}
}

func TestPeople_Get_403Skipped(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	st.jsonRoute("GET", "/apiv2/users/1002/basic", 200, []byte(`{"name":"Bob"}`))
	st.jsonRoute("GET", "/apiv2/users/1002/about", 403, []byte(`{"message":"Forbidden"}`))
	st.jsonRoute("GET", "/apiv2/users/1002/work-contact", 200, []byte(`{}`))
	st.jsonRoute("GET", "/apiv2/users/1002/role", 200, []byte(`{}`))
	if err := runCmd(t, "people", "get", "bob@example.com"); err != nil {
		t.Fatalf("expected success with 403 on one section: %v", err)
	}
	if !strings.Contains(buf.String(), "basic") {
		t.Errorf("expected basic in output: %s", buf.String())
	}
}

func TestPeople_Search(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmd(t, "people", "search", "engineer"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Alice") {
		t.Errorf("expected Alice in engineer search: %s", buf.String())
	}
	if strings.Contains(buf.String(), "Bob") || strings.Contains(buf.String(), "Carol") {
		t.Errorf("non-engineer matches leaked in: %s", buf.String())
	}
}

// ---------- absence ----------

func TestAbsence_List(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/absences/manager", 200, []byte(`{"items":[]}`))
	if err := runCmd(t, "absence", "list"); err != nil {
		t.Fatal(err)
	}
}

func TestAbsence_Policies(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/absence-policies/team/extended", 200, fixturePoliciesExtended)
	if err := runCmd(t, "absence", "policies"); err != nil {
		t.Fatal(err)
	}
}

func TestAbsence_Balance(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/absences/users/leave-days", 200, []byte(`{}`))
	if err := runCmd(t, "absence", "balance"); err != nil {
		t.Fatal(err)
	}
}

func TestAbsence_Check_DryRun(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceVerifyOverlap)
	st.jsonRoute("POST", "/apiv2/absences/request-value-and-balance2", 201, fixtureRequestValueBalance)
	if err := runCmd(t, "absence", "check", "--policy", "512", "--start", "2030-06-01"); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("POST", "/apiv2/absences/multiple")) != 0 {
		t.Error("`absence check` should not POST to /absences/multiple")
	}
}

func TestAbsence_Book(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceVerifyOverlap)
	st.jsonRoute("POST", "/apiv2/absences/request-value-and-balance2", 201, fixtureRequestValueBalance)
	st.jsonRoute("POST", "/apiv2/absences/multiple", 201, fixtureBookSuccess)
	if err := runCmd(t, "absence", "book", "--policy", "512", "--start", "2030-06-01", "--yes"); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("POST", "/apiv2/absences/multiple")) != 1 {
		t.Error("expected /absences/multiple to be called once")
	}
}

func TestAbsence_Book_RequiresFlags(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	if err := runCmd(t, "absence", "book"); err == nil {
		t.Fatal("expected error when --policy / --start missing")
	}
}

// ---------- finance / ops ----------

func TestPayslip_List(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/payrolls", fakeUserID), 200, []byte(`[]`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/payslips", fakeUserID), 200, []byte(`[]`))
	if err := runCmd(t, "payslip", "list"); err != nil {
		t.Fatal(err)
	}
}

func TestEquity_Show_ForOther(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/9999/equity", 200, []byte(`{}`))
	if err := runCmd(t, "equity", "show", "--user", "9999"); err != nil {
		t.Fatal(err)
	}
}

func TestPension_Show(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/employees/%d/pension", fakeUserID), 200, []byte(`{}`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/employees/%d/pension/contributions", fakeUserID), 200, []byte(`{}`))
	if err := runCmd(t, "pension", "show"); err != nil {
		t.Fatal(err)
	}
}

func TestDevice_List(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/devices/users/%d", fakeUserID), 200, []byte(`{}`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/devices/users/%d/in-transit", fakeUserID), 200, []byte(`{}`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/devices/orders/users/%d", fakeUserID), 200, []byte(`{}`))
	if err := runCmd(t, "device", "list"); err != nil {
		t.Fatal(err)
	}
}

func TestAttendance_Show(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.route("GET", "/apiv2/attendance-entries/users/", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	if err := runCmd(t, "attendance", "show"); err != nil {
		t.Fatal(err)
	}
}

func TestCalendar_Show(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/calendar/initial", 200, []byte(`{}`))
	if err := runCmd(t, "calendar", "show"); err != nil {
		t.Fatal(err)
	}
}

func TestExpense_List(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/expense/user/paginated", 200, []byte(`{}`))
	if err := runCmd(t, "expense", "list"); err != nil {
		t.Fatal(err)
	}
}

// ---------- review / goal ----------

func TestReview_List(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/review-cycle/ongoing/parents", 200, []byte(`[]`))
	if err := runCmd(t, "review", "list"); err != nil {
		t.Fatal(err)
	}
}

func TestReview_Get(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	uuid := "uu-id"
	st.jsonRoute("GET", "/apiv2/review-cycle/"+uuid, 200, []byte(`{}`))
	if err := runCmd(t, "review", "get", uuid); err != nil {
		t.Fatal(err)
	}
}

func TestReview_Describe(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	uuid := "uu-id"
	for _, p := range []string{
		"/apiv2/review-cycle/" + uuid,
		"/apiv2/review-result/navigation/" + uuid,
		"/apiv2/review-cycle/progress/" + uuid,
		"/apiv2/review-result/progress/" + uuid,
		"/apiv2/review-result/participation/" + uuid,
	} {
		st.jsonRoute("GET", p, 200, []byte(`{}`))
	}
	if err := runCmd(t, "review", "describe", uuid); err != nil {
		t.Fatal(err)
	}
}

func TestGoal_List_All(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/goals", 200, []byte(`[]`))
	if err := runCmd(t, "goal", "list"); err != nil {
		t.Fatal(err)
	}
}

func TestGoal_List_ForUser(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/goals/user/%d", fakeUserID), 200, []byte(`[]`))
	if err := runCmd(t, "goal", "list", "--user", "me"); err != nil {
		t.Fatal(err)
	}
}

// ---------- company ----------

func TestCompany_Show(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/companies/config", 200, []byte(`{}`))
	if err := runCmd(t, "company", "show"); err != nil {
		t.Fatal(err)
	}
}

func TestCompany_DepartmentList(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/companies/departments", 200, []byte(`[]`))
	if err := runCmd(t, "company", "department", "list"); err != nil {
		t.Fatal(err)
	}
}

// ---------- config + version + raw ----------

func TestConfig_View(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/auth/me", 200, fixtureAuthMe)
	if err := runCmd(t, "config", "view"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), fakeEmail) {
		t.Errorf("expected current user email in config view: %s", buf.String())
	}
}

func TestVersion(t *testing.T) {
	srv, _, _, buf := withTestEnv(t)
	_ = srv
	prev := version
	version = "1.2.3-test"
	defer func() { version = prev }()
	if err := runCmd(t, "version"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "1.2.3-test") {
		t.Errorf("expected 1.2.3-test in output: %q", buf.String())
	}
}

func TestRaw_Get(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, []byte(`{"hit":true}`))
	if err := runCmd(t, "raw", "GET", "/apiv2/users/cache"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "hit") {
		t.Errorf("body not propagated: %s", buf.String())
	}
}

func TestRaw_PostBody(t *testing.T) {
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

// ---------- output formats ----------

func TestOutput_YAML(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/companies/departments", 200, []byte(`[{"id":1,"name":"Eng"}]`))
	if err := runCmd(t, "-o", "yaml", "company", "department", "list"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "name: Eng") {
		t.Errorf("expected yaml `name: Eng`, got: %q", buf.String())
	}
}

func TestOutput_TableDefault(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	if err := runCmdHuman(t, "people", "list"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "NAME") || !strings.Contains(buf.String(), "EMAIL") {
		t.Errorf("expected table headers: %s", buf.String())
	}
}

func TestOutput_Wide(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/users/cache", 200, fixturePeopleCache)
	root := newRootCmd()
	root.SetArgs([]string{"-o", "wide", "--no-cache", "people", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "JOBPOSITION") {
		t.Errorf("expected wide columns (JOBPOSITION), got: %s", buf.String())
	}
}

func TestOutput_InvalidFormat(t *testing.T) {
	srv, _, _, _ := withTestEnv(t)
	_ = srv
	if err := runCmd(t, "-o", "csv", "people", "list"); err == nil {
		t.Fatal("expected error for invalid format")
	}
}

// ---------- label selector ----------

func TestLabelSelector_Parse(t *testing.T) {
	sel, err := parseLabelSelector("dept=Eng,site=London")
	if err != nil {
		t.Fatal(err)
	}
	if sel["dept"] != "eng" || sel["site"] != "london" {
		t.Errorf("bad parse: %#v", sel)
	}
	if _, err := parseLabelSelector("bad"); err == nil {
		t.Error("expected error for malformed selector")
	}
	if _, err := parseLabelSelector("=value"); err == nil {
		t.Error("expected error for empty key")
	}
}

func TestLabelSelector_Match(t *testing.T) {
	sel, _ := parseLabelSelector("department=eng")
	row := map[string]any{"department": "Engineering", "site": "London"}
	if !matchesLabels(row, sel) {
		t.Error("expected `eng` substring to match Engineering value")
	}
	sel, _ = parseLabelSelector("department=sales")
	if matchesLabels(row, sel) {
		t.Error("sales should not match Engineering")
	}
}

// ---------- shared fixtures ----------

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
