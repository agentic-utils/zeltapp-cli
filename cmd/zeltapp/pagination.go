package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

// pageFlags is the standard set wired by addPageFlags.
type pageFlags struct {
	All        bool
	NoPaginate bool
	PageSize   int
	Limit      int
}

// addPageFlags binds --all / --no-paginate / --page-size / --limit to a
// cobra command. Pass the returned pointer into paginate(). The cobra
// PreRunE validates the numeric flags so we never reach the upstream with
// negative values.
func addPageFlags(cmd *cobra.Command, defaultPageSize int) *pageFlags {
	pf := &pageFlags{PageSize: defaultPageSize}
	cmd.Flags().BoolVar(&pf.All, "all", false, "drain all pages (default: single page)")
	cmd.Flags().BoolVar(&pf.NoPaginate, "no-paginate", false, "fetch exactly one page even if more exist")
	cmd.Flags().IntVar(&pf.PageSize, "page-size", defaultPageSize, "items per page (upstream limit)")
	cmd.Flags().IntVar(&pf.Limit, "limit", 0, "cap on total items returned (0 = no cap)")
	prev := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		if pf.PageSize < 1 {
			return fmt.Errorf("--page-size must be >= 1 (got %d)", pf.PageSize)
		}
		if pf.Limit < 0 {
			return fmt.Errorf("--limit must be >= 0 (got %d); use 0 for no cap", pf.Limit)
		}
		if prev != nil {
			return prev(c, args)
		}
		return nil
	}
	return pf
}

// paginate fetches one or more pages of basePath (which is expected to ignore
// page/pageSize query params if it doesn't paginate). qExtra carries the
// non-pagination query params. Returns the concatenated items and the raw
// shape of the first page (so callers can fall back to that if extraction
// fails or pagination was disabled).
//
// The pagination contract is "stop when items < pageSize OR limit reached OR
// --all not passed". Works for Zelt's `?page=N&pageSize=M` style endpoints
// (/absences/manager, /expense/user/paginated).
func paginate(c *client, basePath string, qExtra url.Values, pf *pageFlags) ([]any, json.RawMessage, error) {
	if pf == nil {
		pf = &pageFlags{PageSize: 50}
	}
	pageSize := pf.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	var collected []any
	var firstPage json.RawMessage
	page := 1
	for {
		q := url.Values{}
		for k, vv := range qExtra {
			for _, v := range vv {
				q.Add(k, v)
			}
		}
		q.Set("page", strconv.Itoa(page))
		q.Set("pageSize", strconv.Itoa(pageSize))

		var raw json.RawMessage
		path := basePath
		if encoded := q.Encode(); encoded != "" {
			path = basePath + "?" + encoded
		}
		if err := c.do("GET", path, nil, &raw); err != nil {
			return nil, nil, err
		}
		if page == 1 {
			firstPage = raw
		}

		items, ok := extractPageItems(raw)
		if !ok {
			// Couldn't find a list inside the response — bail out with the raw
			// first-page bytes so callers can still emit something sensible.
			return collected, firstPage, nil
		}
		collected = append(collected, items...)

		if pf.Limit > 0 && len(collected) >= pf.Limit {
			collected = collected[:pf.Limit]
			break
		}
		if pf.NoPaginate || !pf.All {
			break
		}
		if len(items) < pageSize {
			break // last page
		}
		page++
	}
	return collected, firstPage, nil
}

// extractPageItems looks for the rows inside a paginated response.
// Returns (items, true) if recognised, (nil, false) otherwise.
func extractPageItems(raw json.RawMessage) ([]any, bool) {
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, true
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, key := range []string{"items", "data", "results", "rows", "absences"} {
			if v, ok := obj[key]; ok {
				if err := json.Unmarshal(v, &arr); err == nil {
					return arr, true
				}
			}
		}
	}
	return nil, false
}

// emitPaginated renders the result of paginate(). If items are recognised it
// emits them as a list (so -o table / json / yaml all work uniformly); if the
// shape was unrecognised, falls back to the first page's raw bytes.
func emitPaginated(items []any, firstPage json.RawMessage) error {
	if items == nil && len(firstPage) > 0 {
		return emit(&resourceView{raw: rawToAny(firstPage)})
	}
	if len(items) == 0 {
		fmt.Fprintln(outWriter, "(empty)")
		return nil
	}
	return emit(&resourceView{raw: items})
}
