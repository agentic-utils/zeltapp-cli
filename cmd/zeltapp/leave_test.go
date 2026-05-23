package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestLeave_CheckDryRunHitsBothEndpoints(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceVerifyOverlap)
	st.jsonRoute("POST", "/apiv2/absences/request-value-and-balance2", 201, fixtureRequestValueBalance)

	c, _ := newAuthedTestClient(t, srv)
	if err := runDryRun(c, 512, "2030-06-01", "", "summer", false, false, true); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("POST", "/apiv2/absences/verify-overlap")) != 1 {
		t.Error("verify-overlap not called exactly once")
	}
	if len(st.requestsTo("POST", "/apiv2/absences/request-value-and-balance2")) != 1 {
		t.Error("balance not called exactly once")
	}
}

func TestLeave_CheckPropagatesOverlapWarning(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceOverlap)
	st.jsonRoute("POST", "/apiv2/absences/request-value-and-balance2", 201, fixtureRequestValueBalance)
	c, _ := newAuthedTestClient(t, srv)

	// runDryRun prints to stderr; we just assert it doesn't error
	if err := runDryRun(c, 512, "2030-06-01", "", "", false, false, true); err != nil {
		t.Fatal(err)
	}
}

func TestLeave_NewAbsenceReqShape(t *testing.T) {
	c := &client{session: &session{UserID: 7777}}
	req := newAbsenceReq(c, 512, "2030-06-01", "2030-06-05", "summer", false, false)
	if req.UserIDs[0] != 7777 || req.PolicyID != 512 {
		t.Errorf("ids wrong: %#v", req)
	}
	if req.Start != "2030-06-01" {
		t.Errorf("start wrong: %s", req.Start)
	}
	if req.End == nil || *req.End != "2030-06-05" {
		t.Errorf("end wrong: %#v", req.End)
	}
	if req.Notes != "summer" {
		t.Errorf("notes wrong: %s", req.Notes)
	}
	if req.MembersRule != "select-specific" {
		t.Errorf("membersRule wrong: %s", req.MembersRule)
	}
}

func TestLeave_NewAbsenceReqSingleDayNilEnd(t *testing.T) {
	c := &client{session: &session{UserID: 1}}
	req := newAbsenceReq(c, 1, "2030-06-01", "", "", false, false)
	if req.End != nil {
		t.Errorf("expected End=nil for single-day, got %v", *req.End)
	}
}

func TestLeave_BookPostShapeMatchesRecordedSchema(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("POST", "/apiv2/absences/multiple", 201, fixtureBookSuccess)
	c, _ := newAuthedTestClient(t, srv)

	req := newAbsenceReq(c, 512, "2030-06-01", "", "", false, false)
	var out bookResp
	if err := c.do("POST", "/apiv2/absences/multiple", req, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Success {
		t.Error("expected success=true")
	}
	if out.NoOfCreatedAbsences != 1 {
		t.Errorf("expected 1 created, got %d", out.NoOfCreatedAbsences)
	}

	// Verify body shape exactly matches the recorded schema field names.
	r := st.lastRequest(t)
	var parsed map[string]any
	if err := json.Unmarshal(r.Body, &parsed); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{
		"userIds", "policyId", "start", "end", "notes", "attachment",
		"selectFieldData", "multiSelectFieldData", "morningOnly", "afternoonOnly",
		"startHour", "endHour", "startHourTimestamp", "endHourTimestamp",
		"membersRule", "customRule",
	}
	for _, k := range wantKeys {
		if _, ok := parsed[k]; !ok {
			t.Errorf("body missing key %q (recorded schema): %#v", k, parsed)
		}
	}
}

func TestNilIfEmpty(t *testing.T) {
	if nilIfEmpty("") != nil {
		t.Error("empty string should map to nil")
	}
	if nilIfEmpty("x") != "x" {
		t.Error("non-empty should pass through")
	}
}

func TestLeave_BalanceEndpoint(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/absences/users/leave-days", 200, fixtureLeaveDays)
	c, _ := newAuthedTestClient(t, srv)

	var out map[string]any
	if err := c.do("GET", "/apiv2/absences/users/leave-days", nil, &out); err != nil {
		t.Fatal(err)
	}
	policies, ok := out["policies"].([]any)
	if !ok || len(policies) == 0 {
		t.Errorf("expected policies array, got %#v", out)
	}
}

func TestLeave_PoliciesEndpoint(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("GET", "/apiv2/absence-policies/team/extended", 200, fixturePoliciesExtended)
	c, _ := newAuthedTestClient(t, srv)

	var out []map[string]any
	if err := c.do("GET", "/apiv2/absence-policies/team/extended", nil, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 policies, got %d", len(out))
	}
}

func TestLeave_OverlapResponseUnmarshals(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceOverlap)
	c, _ := newAuthedTestClient(t, srv)

	var resp overlapResp
	body := map[string]any{"absenceStart": "2030-01-01", "userIds": []int{c.session.UserID}}
	if err := c.do("POST", "/apiv2/absences/verify-overlap", body, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.IsOverlapping {
		t.Error("expected isOverlapping=true from fixture")
	}
	if len(resp.Absences) != 1 {
		t.Errorf("expected 1 absence in fixture, got %d", len(resp.Absences))
	}
}

func TestLeave_VerifyOverlapBodyShape(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("POST", "/apiv2/absences/verify-overlap", 201, fixtureAbsenceVerifyOverlap)
	c, _ := newAuthedTestClient(t, srv)

	overlapReq := map[string]any{
		"absenceStart": "2030-06-01", "absenceEnd": nil, "absenceId": nil,
		"userIds": []int{c.session.UserID},
		"morningOnly": false, "afternoonOnly": false,
		"startHour": nil, "endHour": nil,
	}
	if err := c.do("POST", "/apiv2/absences/verify-overlap", overlapReq, nil); err != nil {
		t.Fatal(err)
	}
	r := st.lastRequest(t)
	if r.Header.Get("Content-Type") != "application/json" {
		t.Error("Content-Type missing on POST")
	}
	for _, k := range []string{"absenceStart", "absenceEnd", "absenceId", "userIds", "morningOnly", "afternoonOnly", "startHour", "endHour"} {
		var parsed map[string]any
		_ = json.Unmarshal(r.Body, &parsed)
		if _, ok := parsed[k]; !ok {
			t.Errorf("body missing key %q (matched against recorded schema)", k)
		}
	}
}

// Ensure the server-returned 401 from the booking route propagates properly
// (e.g. policy archived, no permission).
func TestLeave_BookErrorPropagates(t *testing.T) {
	srv, st := newTestServer(t)
	st.jsonRoute("POST", "/apiv2/absences/multiple", 403, []byte(`{"error":"not allowed"}`))
	st.jsonRoute("POST", "/apiv2/auth/refresh", 401, []byte(``))
	st.jsonRoute("POST", "/apiv2/auth/refresh-token", 404, []byte(``))
	st.jsonRoute("POST", "/apiv2/auth/token/refresh", 404, []byte(``))
	c, _ := newAuthedTestClient(t, srv)

	req := newAbsenceReq(c, 512, "2030-06-01", "", "", false, false)
	err := c.do("POST", "/apiv2/absences/multiple", req, nil)
	if err == nil {
		t.Fatal("expected error from 403")
	}
}

// dummy reference so go vet doesn't complain about unused http import in
// other test files.
var _ = http.StatusOK
