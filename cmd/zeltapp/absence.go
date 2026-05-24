package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func absenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "absence",
		Aliases: []string{"leave", "abs"},
		Short:   "Absences: list, balance, policies, book, check",
	}
	cmd.AddCommand(
		absenceListCmd(),
		absencePoliciesCmd(),
		absenceBalanceCmd(),
		absenceBookCmd(),
		absenceCheckCmd(),
	)
	return cmd
}

func absenceListCmd() *cobra.Command {
	var calendar string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Team absences",
	}
	pf := addPageFlags(cmd, 50)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := validateOutput(); err != nil {
			return err
		}
		return withClient(func(c *client) error {
			q := url.Values{}
			q.Set("Calendar", calendar)
			items, firstPage, err := paginate(c, "/apiv2/absences/manager", q, pf)
			if err != nil {
				return err
			}
			return emitPaginated(items, firstPage)
		})
	}
	cmd.Flags().StringVar(&calendar, "calendar", "current", "calendar window (past|current|future)")
	return cmd
}

func absencePoliciesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "policies",
		Aliases: []string{"policy"},
		Short:   "Absence policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/absence-policies/team/extended", nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
}

func absenceBalanceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "balance",
		Short: "Leave-day balances",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/absences/users/leave-days", nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
}

func absenceCheckCmd() *cobra.Command {
	var policy int
	var start, end, notes string
	var morning, afternoon bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Dry-run an absence (verify overlap + balance, no booking)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			if err := validateAbsenceFlags(policy, start, end, morning, afternoon); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				return runAbsenceDryRun(c, policy, start, end, notes, morning, afternoon)
			})
		},
	}
	addBookFlags(cmd, &policy, &start, &end, &notes, &morning, &afternoon)
	return cmd
}

// validateAbsenceFlags catches the most common user mistakes locally so we
// don't pay a network round-trip to get a cryptic upstream 400.
func validateAbsenceFlags(policy int, start, end string, morning, afternoon bool) error {
	if policy == 0 || start == "" {
		return errors.New("--policy and --start are required (see `zeltapp absence policies`)")
	}
	if morning && afternoon {
		return errors.New("--morning and --afternoon are mutually exclusive")
	}
	startDay, err := time.Parse("2006-01-02", start)
	if err != nil {
		return fmt.Errorf("--start must be YYYY-MM-DD (got %q)", start)
	}
	if end != "" {
		endDay, err := time.Parse("2006-01-02", end)
		if err != nil {
			return fmt.Errorf("--end must be YYYY-MM-DD (got %q)", end)
		}
		if endDay.Before(startDay) {
			return fmt.Errorf("--end (%s) is before --start (%s)", end, start)
		}
	}
	return nil
}

func absenceBookCmd() *cobra.Command {
	var policy int
	var start, end, notes string
	var morning, afternoon bool
	var yes bool
	cmd := &cobra.Command{
		Use:     "book",
		Aliases: []string{"create"},
		Short:   "Book an absence (runs dry-run + confirmation prompt first unless --yes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			if err := validateAbsenceFlags(policy, start, end, morning, afternoon); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				bal, err := runAbsenceCheck(c, policy, start, end, notes, morning, afternoon)
				if err != nil {
					return err
				}
				if !yes {
					fmt.Fprintf(stderr, "book %s (cost=%v %s, remaining=%v)? [y/N] ",
						formatRange(start, end), bal.RequestValue, bal.Unit, bal.RemainingBalance)
					line, err := bufio.NewReader(stdin).ReadString('\n')
					if err != nil {
						return err
					}
					if strings.TrimSpace(strings.ToLower(line)) != "y" {
						return errors.New("aborted")
					}
				}
				req := newAbsenceRequest(c.session.UserID, policy, start, end, notes, morning, afternoon)
				var out bookResp
				if err := c.do("POST", "/apiv2/absences/multiple", req, &out); err != nil {
					return err
				}
				return emit(&resourceView{raw: out})
			})
		},
	}
	addBookFlags(cmd, &policy, &start, &end, &notes, &morning, &afternoon)
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return cmd
}

func addBookFlags(cmd *cobra.Command, policy *int, start, end, notes *string, morning, afternoon *bool) {
	cmd.Flags().IntVar(policy, "policy", 0, "policyId (see `zeltapp absence policies`)")
	cmd.Flags().StringVar(start, "start", "", "start date YYYY-MM-DD")
	cmd.Flags().StringVar(end, "end", "", "end date YYYY-MM-DD (omit for single-day)")
	cmd.Flags().StringVar(notes, "notes", "", "notes/reason")
	cmd.Flags().BoolVar(morning, "morning", false, "morning half-day only")
	cmd.Flags().BoolVar(afternoon, "afternoon", false, "afternoon half-day only")
}

// ---- shared request types ----

type absenceReq struct {
	UserIDs              []int   `json:"userIds"`
	PolicyID             int     `json:"policyId"`
	Start                string  `json:"start"`
	End                  *string `json:"end"`
	Notes                string  `json:"notes"`
	Attachment           any     `json:"attachment"`
	SelectFieldData      any     `json:"selectFieldData"`
	MultiSelectFieldData any     `json:"multiSelectFieldData"`
	MorningOnly          bool    `json:"morningOnly"`
	AfternoonOnly        bool    `json:"afternoonOnly"`
	StartHour            any     `json:"startHour"`
	EndHour              any     `json:"endHour"`
	StartHourTimestamp   any     `json:"startHourTimestamp"`
	EndHourTimestamp     any     `json:"endHourTimestamp"`
	MembersRule          string  `json:"membersRule"`
	CustomRule           any     `json:"customRule"`
}

type overlapResp struct {
	IsOverlapping bool `json:"isOverlapping"`
}

type balanceResp struct {
	RequestValue     float64 `json:"requestValue"`
	RemainingBalance any     `json:"remainingBalance"`
	Unit             string  `json:"unit"`
}

type bookResp struct {
	Success             bool `json:"success"`
	NoOfCreatedAbsences int  `json:"noOfCreatedAbsences"`
}

func newAbsenceRequest(userID, policy int, start, end, notes string, morning, afternoon bool) *absenceReq {
	var endPtr *string
	if end != "" {
		endPtr = &end
	}
	return &absenceReq{
		UserIDs:       []int{userID},
		PolicyID:      policy,
		Start:         start,
		End:           endPtr,
		Notes:         notes,
		MorningOnly:   morning,
		AfternoonOnly: afternoon,
		MembersRule:   "select-specific",
	}
}

func runAbsenceCheck(c *client, policy int, start, end, notes string, morning, afternoon bool) (*balanceResp, error) {
	overlapBody := map[string]any{
		"absenceStart": start, "absenceEnd": nilIfEmpty(end), "absenceId": nil,
		"userIds":     []int{c.session.UserID},
		"morningOnly": morning, "afternoonOnly": afternoon,
		"startHour": nil, "endHour": nil,
	}
	var overlap overlapResp
	if err := c.do("POST", "/apiv2/absences/verify-overlap", overlapBody, &overlap); err != nil {
		return nil, err
	}
	if overlap.IsOverlapping {
		fmt.Fprintln(stderr, "warning: overlaps with existing absences/events")
	}
	req := newAbsenceRequest(c.session.UserID, policy, start, end, notes, morning, afternoon)
	var bal balanceResp
	if err := c.do("POST", "/apiv2/absences/request-value-and-balance2", req, &bal); err != nil {
		return nil, err
	}
	return &bal, nil
}

func runAbsenceDryRun(c *client, policy int, start, end, notes string, morning, afternoon bool) error {
	overlapBody := map[string]any{
		"absenceStart": start, "absenceEnd": nilIfEmpty(end), "absenceId": nil,
		"userIds":     []int{c.session.UserID},
		"morningOnly": morning, "afternoonOnly": afternoon,
		"startHour": nil, "endHour": nil,
	}
	var overlap overlapResp
	if err := c.do("POST", "/apiv2/absences/verify-overlap", overlapBody, &overlap); err != nil {
		return err
	}
	req := newAbsenceRequest(c.session.UserID, policy, start, end, notes, morning, afternoon)
	var bal balanceResp
	if err := c.do("POST", "/apiv2/absences/request-value-and-balance2", req, &bal); err != nil {
		return err
	}
	return emit(&resourceView{raw: map[string]any{"overlap": overlap, "balance": bal}})
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func formatRange(start, end string) string {
	if end == "" {
		return start
	}
	return start + " to " + end
}
