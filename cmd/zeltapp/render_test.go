package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func withCapturedOut(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := outWriter
	buf := &bytes.Buffer{}
	outWriter = buf
	t.Cleanup(func() { outWriter = prev })
	return buf
}

func TestRender_TableFromSliceOfMaps(t *testing.T) {
	buf := withCapturedOut(t)
	v := []any{
		map[string]any{"id": float64(1), "name": "Alice", "status": "active"},
		map[string]any{"id": float64(2), "name": "Bob", "status": "leave"},
	}
	if err := renderHuman(v); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ID") || !strings.Contains(out, "NAME") || !strings.Contains(out, "STATUS") {
		t.Errorf("expected headers in output, got: %s", out)
	}
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Bob") {
		t.Errorf("expected rows, got: %s", out)
	}
}

func TestRender_KeyValueFromScalarMap(t *testing.T) {
	buf := withCapturedOut(t)
	v := map[string]any{"firstName": "Test", "userId": float64(123), "active": true}
	if err := renderHuman(v); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"firstName", "Test", "userId", "123", "active", "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %s", want, out)
		}
	}
}

func TestRender_EmptyArrayShowsHint(t *testing.T) {
	buf := withCapturedOut(t)
	if err := renderHuman([]any{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(empty)") {
		t.Errorf("expected (empty), got %q", buf.String())
	}
}

func TestRender_NestedMapHasSections(t *testing.T) {
	buf := withCapturedOut(t)
	v := map[string]any{
		"name": "Test",
		"items": []any{
			map[string]any{"id": float64(1), "label": "one"},
		},
	}
	if err := renderHuman(v); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# items") {
		t.Errorf("expected '# items' header, got: %s", out)
	}
	if !strings.Contains(out, "Test") {
		t.Errorf("expected scalar 'Test' in output: %s", out)
	}
}

func TestRender_RawSectionsForFetchMany(t *testing.T) {
	buf := withCapturedOut(t)
	v := map[string]json.RawMessage{
		"basic":    json.RawMessage(`{"firstName":"Test"}`),
		"personal": json.RawMessage(`{"dob":"1990-01-01"}`),
	}
	if err := renderHuman(v); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# basic") || !strings.Contains(out, "# personal") {
		t.Errorf("expected section headers, got: %s", out)
	}
	// basic should come before personal (alphabetical)
	if idx := strings.Index(out, "# basic"); idx == -1 || strings.Index(out, "# personal") < idx {
		t.Errorf("expected alphabetical order: %s", out)
	}
}

func TestRender_FallsBackToJSONForMixed(t *testing.T) {
	buf := withCapturedOut(t)
	// slice of mixed types - can't be rendered as table
	v := []any{"a", float64(1), true}
	if err := renderHuman(v); err != nil {
		t.Fatal(err)
	}
	// Should contain JSON-ish output
	out := buf.String()
	if !strings.Contains(out, `"a"`) {
		t.Errorf("expected JSON fallback, got: %s", out)
	}
}

func TestRender_FormatCellHandlesTypes(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"hello", "hello"},
		{true, "true"},
		{false, "false"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{[]any{1, 2}, "[1,2]"},
	}
	for _, c := range cases {
		if got := formatCell(c.in); got != c.want {
			t.Errorf("formatCell(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRender_IsScalar(t *testing.T) {
	for _, v := range []any{nil, true, "x", float64(1), int(1), int64(1)} {
		if !isScalar(v) {
			t.Errorf("expected scalar for %T", v)
		}
	}
	for _, v := range []any{[]any{}, map[string]any{}} {
		if isScalar(v) {
			t.Errorf("did not expect scalar for %T", v)
		}
	}
}

func TestRender_ColumnPriority(t *testing.T) {
	rows := []map[string]any{
		{"id": float64(1), "name": "x", "type": "a", "zzz": "last"},
		{"id": float64(2), "name": "y", "type": "b", "zzz": "last"},
	}
	cols := chooseColumns(rows)
	if cols[0] != "id" {
		t.Errorf("id should be first, got %v", cols)
	}
	if cols[1] != "name" {
		t.Errorf("name should be second, got %v", cols)
	}
	// zzz should be after the priority columns
	for i, c := range cols {
		if c == "zzz" && i < 3 {
			t.Errorf("zzz should come after priority columns, got %v", cols)
		}
	}
}

func TestRender_TableSkipsNonScalarColumns(t *testing.T) {
	rows := []map[string]any{
		{"id": float64(1), "nested": map[string]any{"x": 1}},
		{"id": float64(2), "nested": map[string]any{"x": 2}},
	}
	cols := chooseColumns(rows)
	for _, c := range cols {
		if c == "nested" {
			t.Errorf("nested column should be skipped, got cols=%v", cols)
		}
	}
}

func TestApiError_ExtractsMessage(t *testing.T) {
	e := &apiError{
		Status: 403, Method: "GET", URL: "/x",
		Body: `{"message":"Forbidden","statusCode":403,"trackingId":"abc"}`,
	}
	msg := e.Error()
	if !strings.Contains(msg, "403") {
		t.Errorf("expected 403 in msg: %s", msg)
	}
	if !strings.Contains(msg, "Forbidden") {
		t.Errorf("expected Forbidden (extracted from message field): %s", msg)
	}
	if strings.Contains(msg, "trackingId") {
		t.Errorf("trackingId should NOT appear in summary msg: %s", msg)
	}
}

func TestApiError_NonJSONBodyTruncated(t *testing.T) {
	e := &apiError{Status: 500, Method: "GET", URL: "/x", Body: strings.Repeat("x", 1000)}
	msg := e.Error()
	if len(msg) > 250 {
		t.Errorf("expected truncated msg, got len=%d", len(msg))
	}
	if !strings.HasSuffix(msg, "...") {
		t.Errorf("expected ... suffix: %s", msg)
	}
}

func TestApiError_UsesErrorFieldWhenMessageMissing(t *testing.T) {
	e := &apiError{
		Status: 400, Method: "POST", URL: "/x",
		Body: `{"error":"something bad"}`,
	}
	if !strings.Contains(e.Error(), "something bad") {
		t.Errorf("expected 'something bad' from error field, got: %s", e.Error())
	}
}
