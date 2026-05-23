package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
)

// renderHuman pretty-prints v in human-readable form. The dispatch is:
//
//   - []any of objects with overlapping keys -> table
//   - map[string]any with only scalar values -> aligned key=value
//   - map[string]json.RawMessage (fetchMany style) -> headed sections
//   - everything else -> falls back to JSON
//
// The fallback means commands stay useful even when the API shape is unfamiliar.
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

// renderHumanMap chooses between key=value and section-header layouts depending
// on whether the values are scalar.
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
		renderKV(scalars)
	}
	if len(sections) > 0 && len(scalars) > 0 {
		fmt.Fprintln(outWriter)
	}
	keys := sortedKeys(sections)
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

// renderRawSections renders a fetchMany-style map (each value is json.RawMessage).
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
	// Only render as table if every element is a map[string]any.
	rows := make([]map[string]any, 0, len(s))
	for _, e := range s {
		m, ok := e.(map[string]any)
		if !ok {
			return renderJSON(s)
		}
		rows = append(rows, m)
	}
	cols := chooseColumns(rows)
	return renderTable(cols, rows)
}

// chooseColumns picks the columns common to most rows, preferring scalar fields
// over nested ones. Output is stable (sorted) so tests can assert ordering.
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
	threshold := (len(rows) + 1) / 2 // appears in at least half
	cols := []string{}
	for k, c := range counts {
		if c >= threshold && scalarOnly[k] {
			cols = append(cols, k)
		}
	}
	if len(cols) == 0 {
		// nothing's reliably scalar -> fall back to all keys
		for k := range counts {
			cols = append(cols, k)
		}
	}
	sort.Slice(cols, func(i, j int) bool {
		// Prefer common id-like fields first, then alphabetic.
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

func renderTable(cols []string, rows []map[string]any) error {
	tw := tabwriter.NewWriter(outWriter, 0, 0, 2, ' ', 0)
	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = strings.ToUpper(c)
	}
	fmt.Fprintln(tw, strings.Join(header, "\t"))
	for _, r := range rows {
		vals := make([]string, len(cols))
		for i, c := range cols {
			vals[i] = formatCell(r[c])
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
	return tw.Flush()
}

func renderKV(m map[string]any) {
	tw := tabwriter.NewWriter(outWriter, 0, 0, 2, ' ', 0)
	keys := sortedKeys(m)
	for _, k := range keys {
		fmt.Fprintf(tw, "%s\t%s\n", k, formatCell(m[k]))
	}
	tw.Flush()
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
		// Drop trailing .0 for whole numbers.
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
