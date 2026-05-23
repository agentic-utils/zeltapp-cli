package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func reviewsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reviews",
		Short: "Performance reviews (cycles, results, entries)",
	}
	cmd.AddCommand(
		reviewsListCmd(),
		reviewsMineCmd(),
		reviewsCycleCmd(),
		reviewsProgressCmd(),
		reviewsParticipationCmd(),
		reviewsResultCmd(),
		reviewsEntryCmd(),
	)
	return cmd
}

func reviewsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List ongoing review cycles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/review-cycle/ongoing/parents", nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	}
}

func reviewsMineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mine",
		Short: "My review results across cycles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				p := fmt.Sprintf("/apiv2/reviews/result/me/%d", c.session.UserID)
				var v json.RawMessage
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	}
}

func reviewsCycleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cycle UUID",
		Short: "Get one review cycle's details + navigation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uuid := args[0]
			return withClient(func(c *client) error {
				out := map[string]json.RawMessage{}
				for label, path := range map[string]string{
					"cycle":      fmt.Sprintf("/apiv2/review-cycle/%s", uuid),
					"navigation": fmt.Sprintf("/apiv2/review-result/navigation/%s", uuid),
				} {
					var v json.RawMessage
					if err := c.do("GET", path, nil, &v); err != nil {
						return err
					}
					out[label] = v
				}
				return printResult(out)
			})
		},
	}
}

func reviewsProgressCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "progress UUID",
		Short: "Progress for a cycle (cycle progress + result progress)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uuid := args[0]
			return withClient(func(c *client) error {
				out := map[string]json.RawMessage{}
				for label, path := range map[string]string{
					"cycleProgress":  fmt.Sprintf("/apiv2/review-cycle/progress/%s", uuid),
					"resultProgress": fmt.Sprintf("/apiv2/review-result/progress/%s", uuid),
				} {
					var v json.RawMessage
					if err := c.do("GET", path, nil, &v); err != nil {
						return err
					}
					out[label] = v
				}
				return printResult(out)
			})
		},
	}
}

func reviewsParticipationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "participation UUID",
		Short: "Who's participating in a cycle",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uuid := args[0]
			return withClient(func(c *client) error {
				p := fmt.Sprintf("/apiv2/review-result/participation/%s", uuid)
				var v json.RawMessage
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	}
}

func reviewsResultCmd() *cobra.Command {
	var forUser int
	cmd := &cobra.Command{
		Use:   "result UUID",
		Short: "User-level result for a cycle (overview + summary). Defaults to self.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uuid := args[0]
			return withClient(func(c *client) error {
				userID := forUser
				if userID == 0 {
					userID = c.session.UserID
				}
				out := map[string]json.RawMessage{}
				for label, path := range map[string]string{
					"cycleUser": fmt.Sprintf("/apiv2/review-cycle/user/%s/%d", uuid, userID),
					"overview":  fmt.Sprintf("/apiv2/review-result/overview/%d/%s", userID, uuid),
					"summary":   fmt.Sprintf("/apiv2/review-result/summary/%d/%s", userID, uuid),
				} {
					var v json.RawMessage
					if err := c.do("GET", path, nil, &v); err != nil {
						return err
					}
					out[label] = v
				}
				return printResult(out)
			})
		},
	}
	cmd.Flags().IntVar(&forUser, "user", 0, "userId (default: self)")
	return cmd
}

func reviewsEntryCmd() *cobra.Command {
	var forUser int
	cmd := &cobra.Command{
		Use:   "entry",
		Short: "Review entry for a user (defaults to self)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				userID := forUser
				if userID == 0 {
					userID = c.session.UserID
				}
				p := fmt.Sprintf("/apiv2/review-entry/%d", userID)
				var v json.RawMessage
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	}
	cmd.Flags().IntVar(&forUser, "user", 0, "userId (default: self)")
	return cmd
}

func goalsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goals",
		Short: "Goals",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "All goals visible to you",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/goals", nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	}, &cobra.Command{
		Use:   "mine",
		Short: "My goals",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				p := fmt.Sprintf("/apiv2/goals/user/%d", c.session.UserID)
				var v json.RawMessage
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	})
	return cmd
}
