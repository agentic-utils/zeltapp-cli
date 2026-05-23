package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type overlapResp struct {
	IsOverlapping bool              `json:"isOverlapping"`
	Absences      []json.RawMessage `json:"absences"`
	Events        []json.RawMessage `json:"events"`
}

type balanceResp struct {
	RequestValue     float64 `json:"requestValue"`
	RemainingBalance any     `json:"remainingBalance"`
	Unit             string  `json:"unit"`
	AccruedByDate    any     `json:"accruedByDate"`
	UsedSoFar        any     `json:"usedSoFar"`
	AccrualDate      any     `json:"accrualDate"`
}

type bookResp struct {
	Success            bool              `json:"success"`
	Method             string            `json:"method"`
	NoOfCreatedAbsences int              `json:"noOfCreatedAbsences"`
	SkippedUsers       []json.RawMessage `json:"skippedUsers"`
}

func leaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leave",
		Short: "List, balance, book and dry-run absences",
	}
	cmd.AddCommand(leaveListCmd(), leaveBalanceCmd(), leavePoliciesCmd(), leaveCheckCmd(), leaveBookCmd())
	return cmd
}

func leaveListCmd() *cobra.Command {
	var pageSize int
	var calendar string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List absences (manager view)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				path := fmt.Sprintf("/apiv2/absences/manager?Calendar=%s&page=1&pageSize=%d", url.QueryEscape(calendar), pageSize)
				var out json.RawMessage
				if err := c.do("GET", path, nil, &out); err != nil {
					return err
				}
				return renderRaw(out)
			})
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "page size")
	cmd.Flags().StringVar(&calendar, "calendar", "current", "calendar window (current|past|future)")
	return cmd
}

func leaveBalanceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "balance",
		Short: "Show leave-day balances",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/absences/users/leave-days", nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	}
}

func leavePoliciesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "policies",
		Short: "List absence policies in your team",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/absence-policies/team/extended", nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	}
}

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

func newAbsenceReq(c *client, policy int, start, end, notes string, morning, afternoon bool) *absenceReq {
	var endPtr *string
	if end != "" {
		endPtr = &end
	}
	return &absenceReq{
		UserIDs:       []int{c.session.UserID},
		PolicyID:      policy,
		Start:         start,
		End:           endPtr,
		Notes:         notes,
		MorningOnly:   morning,
		AfternoonOnly: afternoon,
		MembersRule:   "select-specific",
	}
}

func leaveCheckCmd() *cobra.Command {
	var policy int
	var start, end, notes string
	var morning, afternoon bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Dry-run an absence: verify overlap and compute leave-day cost without booking",
		RunE: func(cmd *cobra.Command, args []string) error {
			if policy == 0 || start == "" {
				return errors.New("--policy and --start required (see `zeltapp leave policies`)")
			}
			return withClient(func(c *client) error {
				return runDryRun(c, policy, start, end, notes, morning, afternoon, true)
			})
		},
	}
	addBookFlags(cmd, &policy, &start, &end, &notes, &morning, &afternoon)
	return cmd
}

func leaveBookCmd() *cobra.Command {
	var policy int
	var start, end, notes string
	var morning, afternoon bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "book",
		Short: "Book absence (runs dry-run + confirmation prompt first unless --yes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if policy == 0 || start == "" {
				return errors.New("--policy and --start required (see `zeltapp leave policies`)")
			}
			return withClient(func(c *client) error {
				if err := runDryRun(c, policy, start, end, notes, morning, afternoon, false); err != nil {
					return err
				}
				if !yes {
					fmt.Fprint(os.Stderr, "\nbook this absence? [y/N]: ")
					line, err := bufio.NewReader(os.Stdin).ReadString('\n')
					if err != nil {
						return err
					}
					if strings.TrimSpace(strings.ToLower(line)) != "y" {
						return errors.New("aborted")
					}
				}
				req := newAbsenceReq(c, policy, start, end, notes, morning, afternoon)
				var out bookResp
				if err := c.do("POST", "/apiv2/absences/multiple", req, &out); err != nil {
					return err
				}
				return printResult(out)
			})
		},
	}
	addBookFlags(cmd, &policy, &start, &end, &notes, &morning, &afternoon)
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return cmd
}

func addBookFlags(cmd *cobra.Command, policy *int, start, end, notes *string, morning, afternoon *bool) {
	cmd.Flags().IntVar(policy, "policy", 0, "policyId (see `zeltapp leave policies`)")
	cmd.Flags().StringVar(start, "start", "", "start date YYYY-MM-DD")
	cmd.Flags().StringVar(end, "end", "", "end date YYYY-MM-DD (omit for single-day)")
	cmd.Flags().StringVar(notes, "notes", "", "notes/reason")
	cmd.Flags().BoolVar(morning, "morning", false, "morning half-day only")
	cmd.Flags().BoolVar(afternoon, "afternoon", false, "afternoon half-day only")
}

func runDryRun(c *client, policy int, start, end, notes string, morning, afternoon, alsoBalance bool) error {
	overlapReq := map[string]any{
		"absenceStart": start, "absenceEnd": nilIfEmpty(end), "absenceId": nil,
		"userIds": []int{c.session.UserID},
		"morningOnly": morning, "afternoonOnly": afternoon,
		"startHour": nil, "endHour": nil,
	}
	var overlap overlapResp
	if err := c.do("POST", "/apiv2/absences/verify-overlap", overlapReq, &overlap); err != nil {
		return err
	}
	if overlap.IsOverlapping {
		fmt.Fprintln(os.Stderr, "warning: overlapping with existing absences/events")
	}

	req := newAbsenceReq(c, policy, start, end, notes, morning, afternoon)
	var bal balanceResp
	if err := c.do("POST", "/apiv2/absences/request-value-and-balance2", req, &bal); err != nil {
		return err
	}
	return printResult(map[string]any{
		"overlap": overlap,
		"balance": bal,
	})
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
