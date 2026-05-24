package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Output formats accepted by the -o flag.
const (
	outputTable = "table"
	outputJSON  = "json"
	outputYAML  = "yaml"
	outputWide  = "wide"
	outputName  = "name" // print only the identifier column(s) - kubectl `-o name`
)

// flagOutput is the global output format ("-o"). Defaults to outputTable.
var flagOutput = outputTable

// validateOutput normalises and validates flagOutput.
func validateOutput() error {
	switch flagOutput {
	case outputTable, outputJSON, outputYAML, outputWide, outputName, "":
		if flagOutput == "" {
			flagOutput = outputTable
		}
		return nil
	}
	return fmt.Errorf("invalid output format %q (want table|json|yaml|wide|name)", flagOutput)
}

// resourceView is what `get` / `describe` commands assemble before printing.
// columns and wideColumns are used in table modes; rows is always populated;
// raw is the unprocessed JSON value used for json/yaml output.
type resourceView struct {
	columns     []string
	wideColumns []string
	rows        []map[string]any
	raw         any
	nameColumn  string // used for `-o name`
}

// emit writes v in whichever format flagOutput specifies.
func emit(v *resourceView) error {
	switch flagOutput {
	case outputJSON:
		return emitJSON(v)
	case outputYAML:
		return emitYAML(v)
	case outputName:
		return emitName(v)
	case outputWide:
		return emitTable(v, append(append([]string{}, v.columns...), v.wideColumns...))
	default:
		return emitTable(v, v.columns)
	}
}

func emitJSON(v *resourceView) error {
	enc := json.NewEncoder(outWriter)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if v.raw != nil {
		return enc.Encode(v.raw)
	}
	return enc.Encode(v.rows)
}

func emitYAML(v *resourceView) error {
	enc := yaml.NewEncoder(outWriter)
	enc.SetIndent(2)
	defer enc.Close()
	if v.raw != nil {
		return enc.Encode(yamlable(v.raw))
	}
	rows := make([]any, len(v.rows))
	for i, r := range v.rows {
		rows[i] = yamlable(r)
	}
	return enc.Encode(rows)
}

// yamlable converts json-decoded values into something yaml.v3 will marshal
// the way you'd expect (map[string]any rather than map[any]any, etc.).
func yamlable(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = yamlable(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = yamlable(val)
		}
		return out
	}
	return v
}

func emitTable(v *resourceView, cols []string) error {
	if len(v.rows) == 0 {
		// scalar/object view: render as key=value
		if m, ok := v.raw.(map[string]any); ok {
			emitKV(m)
			return nil
		}
		// or hand off to renderHuman for raw shapes we didn't structure
		if v.raw != nil {
			return renderHuman(v.raw)
		}
		fmt.Fprintln(outWriter, "(empty)")
		return nil
	}
	if len(cols) == 0 {
		cols = chooseColumns(v.rows)
	}
	tw := tabwriter.NewWriter(outWriter, 0, 0, 2, ' ', 0)
	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = strings.ToUpper(c)
	}
	fmt.Fprintln(tw, strings.Join(header, "\t"))
	for _, r := range v.rows {
		vals := make([]string, len(cols))
		for i, c := range cols {
			vals[i] = formatCell(r[c])
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
	return tw.Flush()
}

func emitKV(m map[string]any) {
	tw := tabwriter.NewWriter(outWriter, 0, 0, 2, ' ', 0)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(tw, "%s\t%s\n", k, formatCell(m[k]))
	}
	tw.Flush()
}

func emitName(v *resourceView) error {
	col := v.nameColumn
	if col == "" {
		col = "name"
	}
	emitted := 0
	for _, r := range v.rows {
		if val, ok := r[col]; ok {
			fmt.Fprintln(outWriter, formatCell(val))
			emitted++
		}
	}
	if emitted > 0 {
		return nil
	}
	// Fallback for resources that populate v.raw (a single record / nested
	// map) instead of v.rows. Pull a sensible identifier out so
	// `zeltapp people get me -o name` isn't a silent zero-byte pipe.
	if v.raw != nil {
		if id := pickIdentifier(v.raw); id != "" {
			fmt.Fprintln(outWriter, id)
			return nil
		}
	}
	return nil
}

// pickIdentifier walks a decoded JSON value looking for the first
// name-like field at any depth. Returns "" if no candidate is found.
func pickIdentifier(v any) string {
	switch val := v.(type) {
	case map[string]any:
		for _, k := range []string{"displayName", "name", "title", "email", "emailAddress", "id", "userId", "uuid"} {
			if x, ok := val[k]; ok {
				if s := formatCell(x); s != "" {
					return s
				}
			}
		}
		// Recurse into nested values (e.g. {"basic": {"displayName": "..."}}).
		for _, x := range val {
			if s := pickIdentifier(x); s != "" {
				return s
			}
		}
	case []any:
		for _, x := range val {
			if s := pickIdentifier(x); s != "" {
				return s
			}
		}
	}
	return ""
}

// labelSelector parses a kubectl-style label selector like
//
//	dept=Engineering,site=London
//
// into a map. Values are matched case-insensitively in matchesLabels.
type labelSelector map[string]string

func parseLabelSelector(s string) (labelSelector, error) {
	if s == "" {
		return nil, nil
	}
	out := labelSelector{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		i := strings.Index(part, "=")
		if i <= 0 {
			return nil, fmt.Errorf("invalid selector %q (want key=value)", part)
		}
		key := strings.TrimSpace(part[:i])
		val := strings.TrimSpace(part[i+1:])
		if key == "" || val == "" {
			return nil, errors.New("selector key and value must be non-empty")
		}
		out[strings.ToLower(key)] = strings.ToLower(val)
	}
	return out, nil
}

// matchesLabels reports whether row satisfies all selectors. Keys are matched
// case-insensitively; values use substring match (so `dept=eng` matches
// `Engineering`).
func matchesLabels(row map[string]any, sel labelSelector) bool {
	for k, want := range sel {
		var got string
		for rk, rv := range row {
			if strings.EqualFold(rk, k) {
				got = strings.ToLower(formatCell(rv))
				break
			}
		}
		if !strings.Contains(got, want) {
			return false
		}
	}
	return true
}
