package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const fakeCycleUUID = "00000000-1111-2222-3333-444455556666"

func TestCmd_ReviewsList(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/review-cycle/ongoing/parents", 200,
		mustJSON([]map[string]any{{"id": fakeCycleUUID, "name": "H1 2030"}}))
	if err := runCmd(t, "reviews", "list"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_ReviewsMine(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/reviews/result/me/%d", fakeUserID), 200, []byte(`{}`))
	if err := runCmd(t, "reviews", "mine"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_ReviewsCycle(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/review-cycle/"+fakeCycleUUID, 200,
		mustJSON(map[string]any{"id": fakeCycleUUID, "name": "H1"}))
	st.jsonRoute("GET", "/apiv2/review-result/navigation/"+fakeCycleUUID, 200,
		mustJSON(map[string]any{"sections": []any{}}))
	if err := runCmd(t, "reviews", "cycle", fakeCycleUUID); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["cycle"]; !ok {
		t.Error("cycle key missing")
	}
	if _, ok := out["navigation"]; !ok {
		t.Error("navigation key missing")
	}
}

func TestCmd_ReviewsProgress(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/review-cycle/progress/"+fakeCycleUUID, 200, []byte(`{}`))
	st.jsonRoute("GET", "/apiv2/review-result/progress/"+fakeCycleUUID, 200, []byte(`{}`))
	if err := runCmd(t, "reviews", "progress", fakeCycleUUID); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_ReviewsParticipation(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/review-result/participation/"+fakeCycleUUID, 200, []byte(`[]`))
	if err := runCmd(t, "reviews", "participation", fakeCycleUUID); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_ReviewsResult_Self(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/review-cycle/user/%s/%d", fakeCycleUUID, fakeUserID), 200, []byte(`{}`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/review-result/overview/%d/%s", fakeUserID, fakeCycleUUID), 200, []byte(`{}`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/review-result/summary/%d/%s", fakeUserID, fakeCycleUUID), 200, []byte(`{}`))
	if err := runCmd(t, "reviews", "result", fakeCycleUUID); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_ReviewsResult_ForOtherUser(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	otherID := 9999
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/review-cycle/user/%s/%d", fakeCycleUUID, otherID), 200, []byte(`{}`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/review-result/overview/%d/%s", otherID, fakeCycleUUID), 200, []byte(`{}`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/review-result/summary/%d/%s", otherID, fakeCycleUUID), 200, []byte(`{}`))
	if err := runCmd(t, "reviews", "result", fakeCycleUUID, "--user", fmt.Sprintf("%d", otherID)); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("GET", fmt.Sprintf("/apiv2/review-cycle/user/%s/%d", fakeCycleUUID, otherID))) != 1 {
		t.Error("expected request for other user")
	}
}

func TestCmd_ReviewsEntry_Self(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/review-entry/%d", fakeUserID), 200, []byte(`{}`))
	if err := runCmd(t, "reviews", "entry"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_ReviewsEntry_ForOtherUser(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/review-entry/9999", 200, []byte(`{}`))
	if err := runCmd(t, "reviews", "entry", "--user", "9999"); err != nil {
		t.Fatal(err)
	}
}

// arg validation is cobra's responsibility, not ours. We don't test it here.
var _ = strings.Contains
