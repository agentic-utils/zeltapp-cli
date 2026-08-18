package main

import (
	"encoding/json"
	"testing"
)

// ---------- review detail / answers ----------

func TestReview_Detail_UsesPluralEntriesPath(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/review-entries/abc-123/detail", 200,
		[]byte(`{"id":"abc-123","questions":[{"id":"q1","questionMain":"What went well?"}]}`))
	if err := runCmd(t, "review", "detail", "abc-123"); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &out); err != nil {
		t.Fatalf("output not JSON: %s", buf.String())
	}
	if out["id"] != "abc-123" {
		t.Errorf("expected entry id in output, got %v", out)
	}
	if len(st.requestsTo("GET", "/apiv2/review-entries/abc-123/detail")) != 1 {
		t.Error("expected exactly one request to the plural review-entries detail path")
	}
}

func TestReview_Answers_EntryAndByQuestion(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", "/apiv2/review-answer/entry/abc-123", 200, []byte(`[]`))
	if err := runCmd(t, "review", "answers", "abc-123"); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("GET", "/apiv2/review-answer/entry/abc-123")) != 1 {
		t.Error("expected request to the entry answers path")
	}

	st.jsonRoute("GET", "/apiv2/review-answer/abc-123/q1/by-question-id", 200, []byte(`{"id":"a1"}`))
	if err := runCmd(t, "review", "answers", "abc-123", "q1"); err != nil {
		t.Fatal(err)
	}
	if len(st.requestsTo("GET", "/apiv2/review-answer/abc-123/q1/by-question-id")) != 1 {
		t.Error("expected request to the by-question-id path")
	}
}
