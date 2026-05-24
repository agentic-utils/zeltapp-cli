package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// renderHuman is used by client.renderRaw and a few fallback paths. It picks a
// reasonable layout based on the value's shape without going through the full
// emit() pipeline.
func renderHuman(v any) error {
	switch val := v.(type) {
	case map[string]any:
		return renderHumanMap(val)
	case map[string]json.RawMessage:
		return renderRawSections(val)
	case []any:
		return renderHumanSlice(val)
	default:
		return renderJSON(v)
	}
}

func renderHumanMap(m map[string]any) error {
	scalars := map[string]any{}
	sections := map[string]any{}
	for k, v := range m {
		if isScalar(v) {
			scalars[k] = v
		} else {
			sections[k] = v
		}
	}
	if len(scalars) > 0 {
		emitKV(scalars)
	}
	if len(sections) > 0 && len(scalars) > 0 {
		fmt.Fprintln(outWriter)
	}
	keys := make([]string, 0, len(sections))
	for k := range sections {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			fmt.Fprintln(outWriter)
		}
		fmt.Fprintf(outWriter, "# %s\n", k)
		if err := renderHuman(sections[k]); err != nil {
			return err
		}
	}
	return nil
}

func renderRawSections(m map[string]json.RawMessage) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			fmt.Fprintln(outWriter)
		}
		fmt.Fprintf(outWriter, "# %s\n", k)
		var v any
		if err := json.Unmarshal(m[k], &v); err != nil {
			outWriter.Write(m[k])
			continue
		}
		if err := renderHuman(v); err != nil {
			return err
		}
	}
	return nil
}

func renderHumanSlice(s []any) error {
	if len(s) == 0 {
		fmt.Fprintln(outWriter, "(empty)")
		return nil
	}
	rows := make([]map[string]any, 0, len(s))
	for _, e := range s {
		m, ok := e.(map[string]any)
		if !ok {
			return renderJSON(s)
		}
		rows = append(rows, m)
	}
	cols := chooseColumns(rows)
	return emitTable(&resourceView{rows: rows, columns: cols}, cols)
}

// chooseColumns picks scalar columns common to most rows. Output is sorted
// deterministically (priority fields first, then alphabetical).
func chooseColumns(rows []map[string]any) []string {
	counts := map[string]int{}
	scalarOnly := map[string]bool{}
	for _, r := range rows {
		for k, v := range r {
			counts[k]++
			if isScalar(v) {
				if _, ok := scalarOnly[k]; !ok {
					scalarOnly[k] = true
				}
			} else {
				scalarOnly[k] = false
			}
		}
	}
	threshold := (len(rows) + 1) / 2
	cols := []string{}
	for k, c := range counts {
		if c >= threshold && scalarOnly[k] {
			cols = append(cols, k)
		}
	}
	if len(cols) == 0 {
		for k := range counts {
			cols = append(cols, k)
		}
	}
	sort.Slice(cols, func(i, j int) bool {
		pi, pj := priority(cols[i]), priority(cols[j])
		if pi != pj {
			return pi < pj
		}
		return cols[i] < cols[j]
	})
	return cols
}

func priority(col string) int {
	switch strings.ToLower(col) {
	case "id":
		return 0
	case "userid", "user_id", "policyid", "policy_id", "companyid", "company_id":
		return 1
	case "name", "title", "displayname":
		return 2
	case "type", "status":
		return 3
	}
	return 10
}

func isScalar(v any) bool {
	switch v.(type) {
	case nil, bool, string, float64, float32, int, int32, int64, json.Number:
		return true
	}
	return false
}

func formatCell(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", val), "0"), ".")
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// renderJSON is the canonical pretty-JSON writer (also used as a fallback).
func renderJSON(v any) error {
	enc := json.NewEncoder(outWriter)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
