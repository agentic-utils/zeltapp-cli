package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

func deviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "device",
		Aliases: []string{"devices", "dev"},
		Short:   "IT devices assigned to a user",
	}
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Assigned devices + orders + in-transit (default: me)",
		RunE: func(cmd *cobra.Command, args []string) error {
			user, _ := cmd.Flags().GetString("user")
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				id, err := resolveUserOrSelf(c, user)
				if err != nil {
					return err
				}
				out := map[string]json.RawMessage{}
				for _, path := range []string{
					fmt.Sprintf("/apiv2/devices/users/%d", id),
					fmt.Sprintf("/apiv2/devices/users/%d/in-transit", id),
					fmt.Sprintf("/apiv2/devices/orders/users/%d", id),
				} {
					var v json.RawMessage
					if err := c.do("GET", path, nil, &v); err != nil {
						return err
					}
					out[path] = v
				}
				return emit(&resourceView{raw: rawMapToAny(out)})
			})
		},
	}
	listCmd.Flags().String("user", "me", "userId, email, or 'me'")
	cmd.AddCommand(listCmd)
	return cmd
}

func attendanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attendance",
		Aliases: []string{"att"},
		Short:   "Attendance entries",
	}
	var date string
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Today's attendance widget (default: me, today)",
		RunE: func(cmd *cobra.Command, args []string) error {
			user, _ := cmd.Flags().GetString("user")
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				id, err := resolveUserOrSelf(c, user)
				if err != nil {
					return err
				}
				if date == "" {
					date = c.now().Format("2006-01-02")
				}
				if _, err := time.Parse("2006-01-02", date); err != nil {
					return fmt.Errorf("--date must be YYYY-MM-DD (got %q)", date)
				}
				q := url.Values{}
				q.Set("date", date)
				var v json.RawMessage
				p := fmt.Sprintf("/apiv2/attendance-entries/users/%d/widget?%s", id, q.Encode())
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
	showCmd.Flags().String("user", "me", "userId, email, or 'me'")
	showCmd.Flags().StringVar(&date, "date", "", "date YYYY-MM-DD (default: today)")
	cmd.AddCommand(showCmd)
	return cmd
}

func calendarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "calendar",
		Aliases: []string{"cal"},
		Short:   "Team calendar (absences + schedule + events + holidays)",
	}
	var start, end string
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Team calendar window (default: this week)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				now := c.now()
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
				q.Set("view", "team")
				q.Set("page", "1")
				q.Set("pageSize", "50")
				q.Set("Lifecycle status", "Employed")
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/calendar/initial?"+q.Encode(), nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
	showCmd.Flags().StringVar(&start, "start", "", "start date YYYY-MM-DD")
	showCmd.Flags().StringVar(&end, "end", "", "end date YYYY-MM-DD")
	cmd.AddCommand(showCmd)
	return cmd
}
