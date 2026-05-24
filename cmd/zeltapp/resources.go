package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Helpers shared by the get/describe verbs for the cmd/people/users surface.
// Kept in a single file so deleting an individual command file doesn't break
// callers.

type peopleCacheEntry struct {
	UserID        int    `json:"userId"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	DisplayName   string `json:"displayName"`
	EmailAddress  string `json:"emailAddress"`
	JobPosition   string `json:"jobPosition"`
	Department    string `json:"department"`
	Site          string `json:"site"`
	StartDate     string `json:"startDate"`
	Lifecycle     string `json:"lifecycleStatus"`
	AccountStatus string `json:"accountStatus"`
}

// fetchPeopleCache hits /apiv2/users/cache (long TTL cached) and returns a
// typed view. If the body is some other shape, returns (nil, raw, nil) so
// callers can fall back to printing the raw JSON.
func fetchPeopleCache(c *client) ([]peopleCacheEntry, json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.do("GET", "/apiv2/users/cache", nil, &raw); err != nil {
		return nil, nil, err
	}
	var arr []peopleCacheEntry
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr, raw, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, key := range []string{"users", "data", "items"} {
			if v, ok := obj[key]; ok {
				if err := json.Unmarshal(v, &arr); err == nil {
					return arr, raw, nil
				}
			}
		}
	}
	return nil, raw, nil
}

func filterActive(in []peopleCacheEntry) []peopleCacheEntry {
	out := make([]peopleCacheEntry, 0, len(in))
	for _, p := range in {
		if p.AccountStatus != "" && p.AccountStatus != "Active" {
			continue
		}
		if p.Lifecycle != "" {
			s := strings.ToLower(p.Lifecycle)
			if s == "terminated" || s == "leaver" || s == "deleted" {
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

// rawToAny decodes a json.RawMessage into a generic Go value. On decode error
// the original bytes are returned as a string so the caller can still emit.
func rawToAny(raw json.RawMessage) any {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

// rawMapToAny is rawToAny for a fetchMany-style map.
func rawMapToAny(m map[string]json.RawMessage) any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = rawToAny(v)
	}
	return out
}

// fetchManyForUserTolerant is like fetchManyForUser but swallows 403/404 for
// individual sections so a caller can build a partial view. Returns the
// successful sections plus a list of paths that were skipped.
func fetchManyForUserTolerant(c *client, userID int, strict bool, paths ...string) (map[string]json.RawMessage, []string, error) {
	out := map[string]json.RawMessage{}
	var skipped []string
	for _, p := range paths {
		var v json.RawMessage
		full := fmt.Sprintf("/apiv2/users/%d/%s", userID, p)
		err := c.do("GET", full, nil, &v)
		if err != nil {
			var ae *apiError
			if !strict && errors.As(err, &ae) && (ae.Status == 403 || ae.Status == 404) {
				skipped = append(skipped, p)
				continue
			}
			return nil, nil, err
		}
		out[p] = v
	}
	return out, skipped, nil
}
