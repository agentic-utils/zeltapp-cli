package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func attendanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attendance",
		Short: "Attendance widgets and tables",
	}
	cmd.AddCommand(attendanceTodayCmd(), attendanceTableCmd(), attendanceSettingsCmd())
	return cmd
}

func attendanceTodayCmd() *cobra.Command {
	var date string
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Today's attendance widget",
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			return withClient(func(c *client) error {
				path := fmt.Sprintf("/apiv2/attendance-entries/users/%d/widget?date=%s", c.session.UserID, date)
				var out json.RawMessage
				if err := c.do("GET", path, nil, &out); err != nil {
					return err
				}
				return renderRaw(out)
			})
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "date YYYY-MM-DD (default: today)")
	return cmd
}

func attendanceTableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "table",
		Short: "Full attendance entries (by user)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				path := fmt.Sprintf("/apiv2/attendance-entries/users/%d/table/by-user", c.session.UserID)
				var out json.RawMessage
				if err := c.do("GET", path, nil, &out); err != nil {
					return err
				}
				return renderRaw(out)
			})
		},
	}
}

func attendanceSettingsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "settings",
		Short: "Company-wide attendance settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				var out json.RawMessage
				if err := c.do("GET", "/apiv2/attendance-settings", nil, &out); err != nil {
					return err
				}
				return renderRaw(out)
			})
		},
	}
}
