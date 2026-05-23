package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// peopleCacheEntry is a tolerant view of /apiv2/users/cache rows. Zelt may
// return more fields; we only project the ones useful for listing.
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

func peopleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "people",
		Short: "Browse the company directory",
	}
	cmd.AddCommand(peopleListCmd(), peopleSearchCmd(), peopleGetCmd())
	return cmd
}

// fetchPeopleCache pulls the cached list of all users. The endpoint returns
// either an array directly or an object whose values are arrays - we handle both.
func fetchPeopleCache(c *client) ([]peopleCacheEntry, json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.do("GET", "/apiv2/users/cache", nil, &raw); err != nil {
		return nil, nil, err
	}
	// Try direct array first.
	var arr []peopleCacheEntry
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr, raw, nil
	}
	// Maybe it's an object with a `users` or `data` key.
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
	// Couldn't project - return raw and let caller fall back to JSON output.
	return nil, raw, nil
}

func peopleListCmd() *cobra.Command {
	var includeInactive bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List everyone in the company (active by default)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				arr, raw, err := fetchPeopleCache(c)
				if err != nil {
					return err
				}
				if arr == nil {
					// Unknown shape - just print the JSON.
					return renderRaw(raw)
				}
				if !includeInactive {
					arr = filterActive(arr)
				}
				return printResult(toPresentable(arr))
			})
		},
	}
	cmd.Flags().BoolVar(&includeInactive, "all", false, "include inactive/terminated users")
	return cmd
}

func peopleSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search the directory by name, email, department or job title (case-insensitive substring)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.ToLower(args[0])
			return withClient(func(c *client) error {
				arr, raw, err := fetchPeopleCache(c)
				if err != nil {
					return err
				}
				if arr == nil {
					return renderRaw(raw)
				}
				matched := make([]peopleCacheEntry, 0, len(arr))
				for _, p := range arr {
					if matches(p, query) {
						matched = append(matched, p)
					}
				}
				if len(matched) == 0 {
					fmt.Fprintln(outWriter, "(no matches)")
					return nil
				}
				return printResult(toPresentable(matched))
			})
		},
	}
	return cmd
}

func peopleGetCmd() *cobra.Command {
	var sections []string
	cmd := &cobra.Command{
		Use:   "get ID_OR_EMAIL",
		Short: "Get one person's profile by userId or email",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				id, err := resolveUserID(c, args[0])
				if err != nil {
					return err
				}
				if len(sections) == 0 {
					sections = []string{"basic", "personal", "role", "work-contact"}
				}
				out, err := fetchManyForUser(c, id, sections...)
				if err != nil {
					return err
				}
				return printResult(out)
			})
		},
	}
	cmd.Flags().StringSliceVar(&sections, "sections", nil,
		"comma-separated subpaths under /apiv2/users/<id>/ (default: basic,personal,role,work-contact)")
	return cmd
}

// resolveUserID accepts either a numeric ID or an email. For an email we look
// up via the cached user list.
func resolveUserID(c *client, idOrEmail string) (int, error) {
	if id, err := strconv.Atoi(idOrEmail); err == nil {
		return id, nil
	}
	wantEmail := strings.ToLower(idOrEmail)
	arr, _, err := fetchPeopleCache(c)
	if err != nil {
		return 0, err
	}
	for _, p := range arr {
		if strings.ToLower(p.EmailAddress) == wantEmail {
			return p.UserID, nil
		}
	}
	return 0, errors.New("no user found for " + idOrEmail)
}

func filterActive(in []peopleCacheEntry) []peopleCacheEntry {
	out := make([]peopleCacheEntry, 0, len(in))
	for _, p := range in {
		// Treat AccountStatus=Active and Lifecycle in {Hired,Employed} as active.
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

func matches(p peopleCacheEntry, q string) bool {
	fields := []string{p.FirstName, p.LastName, p.DisplayName, p.EmailAddress,
		p.JobPosition, p.Department, p.Site}
	for _, f := range fields {
		if f != "" && strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

// toPresentable converts the typed entries to []map[string]any so the human
// renderer treats it as a table. Empty fields are omitted so the table stays narrow.
func toPresentable(arr []peopleCacheEntry) []any {
	out := make([]any, 0, len(arr))
	for _, p := range arr {
		row := map[string]any{
			"userId":       p.UserID,
			"name":         p.DisplayName,
			"email":        p.EmailAddress,
			"jobPosition":  p.JobPosition,
			"department":   p.Department,
		}
		if row["name"] == "" {
			row["name"] = strings.TrimSpace(p.FirstName + " " + p.LastName)
		}
		out = append(out, row)
	}
	return out
}
