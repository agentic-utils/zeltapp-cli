package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func peopleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "people",
		Aliases: []string{"person", "user", "users", "po"},
		Short:   "Company directory",
	}
	cmd.AddCommand(peopleListCmd(), peopleGetCmd(), peopleSearchCmd())
	return cmd
}

func peopleListCmd() *cobra.Command {
	var (
		includeInactive bool
		selector        string
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List active company members (--include-inactive for everyone)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				return listPeople(c, includeInactive, selector)
			})
		},
	}
	// Note: --all is reserved cli-wide for pagination drain (review #2). Use
	// --include-inactive here to avoid the flag-name collision.
	cmd.Flags().BoolVarP(&includeInactive, "include-inactive", "A", false, "include inactive/terminated users")
	cmd.Flags().StringVarP(&selector, "selector", "l", "", "label selector, e.g. department=Engineering,site=London")
	return cmd
}

func peopleGetCmd() *cobra.Command {
	var sections []string
	var strict bool
	cmd := &cobra.Command{
		Use:   "get [ID|EMAIL|me]",
		Short: "Get one person's accessible sections (default: me)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			target := "me"
			if len(args) == 1 {
				target = args[0]
			}
			return withClient(func(c *client) error {
				id, err := resolveUserOrSelf(c, target)
				if err != nil {
					return err
				}
				wanted := sections
				if len(wanted) == 0 {
					if id == c.session.UserID {
						wanted = []string{
							"basic", "personal", "about", "address",
							"work-contact", "emergency-contact", "family",
							"role", "contracts", "compensation",
							"bank-accounts", "equity", "lifecycle",
						}
					} else {
						wanted = []string{"basic", "about", "work-contact", "role"}
					}
				}
				out, skipped, err := fetchManyForUserTolerant(c, id, strict, wanted...)
				if err != nil {
					return err
				}
				if len(skipped) > 0 {
					fmt.Fprintf(stderr, "skipped (no access): %s\n", strings.Join(skipped, ", "))
				}
				return emit(&resourceView{raw: rawMapToAny(out)})
			})
		},
	}
	cmd.Flags().StringSliceVar(&sections, "sections", nil,
		"comma-separated subpaths under /apiv2/users/<id>/ (default: sensible per-user set)")
	cmd.Flags().BoolVar(&strict, "strict", false,
		"fail on the first 403/404 instead of skipping the section")
	return cmd
}

func peopleSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search QUERY",
		Short: "Substring search across name, email, department, job title",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			query := strings.ToLower(args[0])
			return withClient(func(c *client) error {
				arr, raw, err := fetchPeopleCache(c)
				if err != nil {
					return err
				}
				if arr == nil {
					return emit(&resourceView{raw: rawToAny(raw)})
				}
				rows := make([]map[string]any, 0, len(arr))
				for _, p := range arr {
					if matchesQuery(p, query) {
						rows = append(rows, peopleRow(p))
					}
				}
				return emit(&resourceView{
					columns:     []string{"userId", "name", "email", "department"},
					wideColumns: []string{"jobPosition", "site", "startDate", "status"},
					rows:        rows,
					nameColumn:  "name",
				})
			})
		},
	}
}

// ---- shared helpers ----

func listPeople(c *client, includeInactive bool, selector string) error {
	arr, raw, err := fetchPeopleCache(c)
	if err != nil {
		return err
	}
	if arr == nil {
		return emit(&resourceView{raw: rawToAny(raw)})
	}
	if !includeInactive {
		arr = filterActive(arr)
	}
	sel, err := parseLabelSelector(selector)
	if err != nil {
		return err
	}
	rows := make([]map[string]any, 0, len(arr))
	for _, p := range arr {
		row := peopleRow(p)
		if sel != nil && !matchesLabels(row, sel) {
			continue
		}
		rows = append(rows, row)
	}
	return emit(&resourceView{
		columns:     []string{"userId", "name", "email", "department"},
		wideColumns: []string{"jobPosition", "site", "startDate", "status"},
		rows:        rows,
		nameColumn:  "name",
	})
}

func peopleRow(p peopleCacheEntry) map[string]any {
	name := p.DisplayName
	if name == "" {
		name = strings.TrimSpace(p.FirstName + " " + p.LastName)
	}
	status := p.Lifecycle
	if status == "" {
		status = p.AccountStatus
	}
	return map[string]any{
		"userId":      p.UserID,
		"name":        name,
		"email":       p.EmailAddress,
		"jobPosition": p.JobPosition,
		"department":  p.Department,
		"site":        p.Site,
		"startDate":   p.StartDate,
		"status":      status,
	}
}

func matchesQuery(p peopleCacheEntry, q string) bool {
	fields := []string{p.FirstName, p.LastName, p.DisplayName, p.EmailAddress,
		p.JobPosition, p.Department, p.Site}
	for _, f := range fields {
		if f != "" && strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

// resolveUserOrSelf maps "me"/empty -> current user, numeric -> id, otherwise
// looks up the email in the cached people directory. When more than one
// person matches (re-hired users, shared mailing lists), it errors with the
// candidate list rather than picking the first arbitrarily.
func resolveUserOrSelf(c *client, target string) (int, error) {
	if target == "" || target == "me" || target == "self" {
		return c.session.UserID, nil
	}
	if id, err := strconv.Atoi(target); err == nil {
		return id, nil
	}
	wantEmail := strings.ToLower(target)
	arr, _, err := fetchPeopleCache(c)
	if err != nil {
		return 0, err
	}
	matches := []peopleCacheEntry{}
	for _, p := range arr {
		if strings.ToLower(p.EmailAddress) == wantEmail {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return 0, errors.New("no user found for " + target)
	case 1:
		return matches[0].UserID, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ambiguous: %d users match %q — disambiguate by userId:\n", len(matches), target)
	for _, p := range matches {
		fmt.Fprintf(&b, "  %d  %s  (%s)\n", p.UserID, p.DisplayName, p.Lifecycle)
	}
	return 0, errors.New(b.String())
}
