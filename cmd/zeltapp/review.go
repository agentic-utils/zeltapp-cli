package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func reviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "review",
		Aliases: []string{"reviews", "rev"},
		Short:   "Performance review cycles, results, entries",
	}
	cmd.AddCommand(
		reviewListCmd(),
		reviewGetCmd(),
		reviewDescribeCmd(),
		reviewProgressCmd(),
		reviewParticipationCmd(),
		reviewResultCmd(),
		reviewEntryCmd(),
	)
	return cmd
}

func reviewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Ongoing review cycles",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/review-cycle/ongoing/parents", nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
}

func reviewGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get UUID",
		Short: "One review cycle by uuid",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/review-cycle/"+args[0], nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
}

func reviewDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe UUID",
		Short: "Cycle details + navigation + progress + participation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			uuid := args[0]
			return withClient(func(c *client) error {
				out := map[string]json.RawMessage{}
				for label, path := range map[string]string{
					"cycle":          "/apiv2/review-cycle/" + uuid,
					"navigation":     "/apiv2/review-result/navigation/" + uuid,
					"cycleProgress":  "/apiv2/review-cycle/progress/" + uuid,
					"resultProgress": "/apiv2/review-result/progress/" + uuid,
					"participation":  "/apiv2/review-result/participation/" + uuid,
				} {
					var v json.RawMessage
					if err := c.do("GET", path, nil, &v); err != nil {
						return err
					}
					out[label] = v
				}
				return emit(&resourceView{raw: rawMapToAny(out)})
			})
		},
	}
}

func reviewProgressCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "progress UUID",
		Short: "Cycle + result progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			uuid := args[0]
			return withClient(func(c *client) error {
				out := map[string]json.RawMessage{}
				for label, path := range map[string]string{
					"cycleProgress":  "/apiv2/review-cycle/progress/" + uuid,
					"resultProgress": "/apiv2/review-result/progress/" + uuid,
				} {
					var v json.RawMessage
					if err := c.do("GET", path, nil, &v); err != nil {
						return err
					}
					out[label] = v
				}
				return emit(&resourceView{raw: rawMapToAny(out)})
			})
		},
	}
}

func reviewParticipationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "participation UUID",
		Short: "Cycle participants",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", "/apiv2/review-result/participation/"+args[0], nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
}

func reviewResultCmd() *cobra.Command {
	var user string
	cmd := &cobra.Command{
		Use:   "result UUID",
		Short: "User-level result for a cycle (default: me)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			uuid := args[0]
			return withClient(func(c *client) error {
				id, err := resolveUserOrSelf(c, user)
				if err != nil {
					return err
				}
				out := map[string]json.RawMessage{}
				for label, path := range map[string]string{
					"cycleUser": fmt.Sprintf("/apiv2/review-cycle/user/%s/%d", uuid, id),
					"overview":  fmt.Sprintf("/apiv2/review-result/overview/%d/%s", id, uuid),
					"summary":   fmt.Sprintf("/apiv2/review-result/summary/%d/%s", id, uuid),
				} {
					var v json.RawMessage
					if err := c.do("GET", path, nil, &v); err != nil {
						return err
					}
					out[label] = v
				}
				return emit(&resourceView{raw: rawMapToAny(out)})
			})
		},
	}
	cmd.Flags().StringVar(&user, "user", "me", "userId, email, or 'me'")
	return cmd
}

func reviewEntryCmd() *cobra.Command {
	var user string
	cmd := &cobra.Command{
		Use:   "entry",
		Short: "Review entry for a user (default: me)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				id, err := resolveUserOrSelf(c, user)
				if err != nil {
					return err
				}
				var v json.RawMessage
				if err := c.do("GET", fmt.Sprintf("/apiv2/review-entry/%d", id), nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
	cmd.Flags().StringVar(&user, "user", "me", "userId, email, or 'me'")
	return cmd
}

func goalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "goal",
		Aliases: []string{"goals"},
		Short:   "Goals",
	}
	var user string
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List goals (omit --user for all visible)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				path := "/apiv2/goals"
				if user != "" {
					id, err := resolveUserOrSelf(c, user)
					if err != nil {
						return err
					}
					path = fmt.Sprintf("/apiv2/goals/user/%d", id)
				}
				var v json.RawMessage
				if err := c.do("GET", path, nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
	listCmd.Flags().StringVar(&user, "user", "", "userId, email, or 'me' (omit for all)")
	cmd.AddCommand(listCmd)
	return cmd
}
