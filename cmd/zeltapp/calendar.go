package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

func calendarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Team / personal calendar (absences, work-schedule, events, holidays)",
	}
	cmd.AddCommand(calendarTeamCmd())
	return cmd
}

func calendarTeamCmd() *cobra.Command {
	var start, end, view string
	var pageSize int
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Team calendar (default: this week)",
		RunE: func(cmd *cobra.Command, args []string) error {
			now := time.Now()
			if start == "" {
				start = now.AddDate(0, 0, -int(now.Weekday())).Format("2006-01-02")
			}
			if end == "" {
				end = now.AddDate(0, 0, 6-int(now.Weekday())).Format("2006-01-02")
			}
			q := url.Values{}
			q.Set("displayFilter", "Absence,Work schedule,Events,Public holiday")
			q.Set("start", start)
			q.Set("end", end)
			q.Set("view", view)
			q.Set("page", "1")
			q.Set("pageSize", fmt.Sprintf("%d", pageSize))
			q.Set("Lifecycle status", "Employed")
			return withClient(func(c *client) error {
				var out json.RawMessage
				if err := c.do("GET", "/apiv2/calendar/initial?"+q.Encode(), nil, &out); err != nil {
					return err
				}
				return renderRaw(out)
			})
		},
	}
	cmd.Flags().StringVar(&start, "start", "", "start date YYYY-MM-DD (default: Sunday this week)")
	cmd.Flags().StringVar(&end, "end", "", "end date YYYY-MM-DD (default: Saturday this week)")
	cmd.Flags().StringVar(&view, "view", "team", "view (team|individual)")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "page size")
	return cmd
}
