package main

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

// finance.go groups money/benefits-shaped resources, each of which only has
// one or two verbs.

func payslipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "payslip",
		Aliases: []string{"payslips", "payroll", "ps"},
		Short:   "Payrolls + payslips",
	}
	cmd.AddCommand(simpleUserScoped("list", "payrolls + payslips for a user (default: me)",
		[]string{"basic"}, func(id int) []string {
			return []string{
				fmt.Sprintf("/apiv2/users/%d/payrolls", id),
				fmt.Sprintf("/apiv2/users/%d/payslips", id),
			}
		}))
	return cmd
}

func compensationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "compensation",
		Aliases: []string{"comp", "salary"},
		Short:   "Compensation history",
	}
	cmd.AddCommand(simpleUserScopedSingle("show", "compensation for a user (default: me)", "compensation"))
	return cmd
}

func equityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "equity",
		Short: "Equity grants",
	}
	cmd.AddCommand(simpleUserScopedSingle("show", "equity grants for a user (default: me)", "equity"))
	return cmd
}

func pensionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pension",
		Aliases: []string{"pensions"},
		Short:   "Pension scheme + contributions",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Pension scheme + contributions (default: me)",
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
				for _, suffix := range []string{"", "/contributions"} {
					var v json.RawMessage
					p := fmt.Sprintf("/apiv2/employees/%d/pension%s", id, suffix)
					if err := c.do("GET", p, nil, &v); err != nil {
						return err
					}
					key := "pension"
					if suffix != "" {
						key = "contributions"
					}
					out[key] = v
				}
				return emit(&resourceView{raw: rawMapToAny(out)})
			})
		},
	})
	cmd.PersistentFlags().String("user", "me", "userId, email, or 'me'")
	return cmd
}

func benefitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "benefit",
		Aliases: []string{"benefits"},
		Short:   "Active custom benefits",
	}
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Active custom benefits for a user (default: me)",
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
				var v json.RawMessage
				p := fmt.Sprintf("/apiv2/custom-benefit/by-user/%d/effective", id)
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
	listCmd.Flags().String("user", "me", "userId, email, or 'me'")
	cmd.AddCommand(listCmd)
	return cmd
}

func contractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "contract",
		Aliases: []string{"contracts"},
		Short:   "Employment contracts",
	}
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "All contracts (history + current) for a user (default: me)",
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
				out, _, err := fetchManyForUserTolerant(c, id, false, "contracts", "contracts/current")
				if err != nil {
					return err
				}
				return emit(&resourceView{raw: rawMapToAny(out)})
			})
		},
	}
	listCmd.Flags().String("user", "me", "userId, email, or 'me'")

	currentCmd := &cobra.Command{
		Use:   "current",
		Short: "Current contract only",
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
				var v json.RawMessage
				p := fmt.Sprintf("/apiv2/users/%d/contracts/current", id)
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
	currentCmd.Flags().String("user", "me", "userId, email, or 'me'")
	cmd.AddCommand(listCmd, currentCmd)
	return cmd
}

func expenseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "expense",
		Aliases: []string{"expenses", "exp"},
		Short:   "Expense claims",
	}
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List expense claims",
	}
	pf := addPageFlags(listCmd, 50)
	listCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := validateOutput(); err != nil {
			return err
		}
		return withClient(func(c *client) error {
			items, firstPage, err := paginate(c, "/apiv2/expense/user/paginated", url.Values{}, pf)
			if err != nil {
				return err
			}
			return emitPaginated(items, firstPage)
		})
	}
	cmd.AddCommand(listCmd)
	return cmd
}

func invoiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "invoice",
		Aliases: []string{"invoices", "inv"},
		Short:   "Contractor invoices",
	}
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List contractor invoices for a user (default: me)",
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
				var v json.RawMessage
				p := fmt.Sprintf("/apiv2/contractor/invoice/users/%d", id)
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
	listCmd.Flags().String("user", "me", "userId, email, or 'me'")
	cmd.AddCommand(listCmd)
	return cmd
}

// simpleUserScoped builds a subcommand that fans out N URLs for a given user.
// pathsFor returns the absolute URLs given an integer userId. The verb name
// (e.g. "list", "show") is configurable.
func simpleUserScoped(use, short string, _ []string, pathsFor func(id int) []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     use,
		Aliases: []string{"ls"},
		Short:   short,
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
				for _, p := range pathsFor(id) {
					var v json.RawMessage
					if err := c.do("GET", p, nil, &v); err != nil {
						return err
					}
					out[p] = v
				}
				return emit(&resourceView{raw: rawMapToAny(out)})
			})
		},
	}
	cmd.Flags().String("user", "me", "userId, email, or 'me'")
	return cmd
}

// simpleUserScopedSingle is the single-path variant.
func simpleUserScopedSingle(use, short, suffix string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
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
				var v json.RawMessage
				p := fmt.Sprintf("/apiv2/users/%d/%s", id, suffix)
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
	cmd.Flags().String("user", "me", "userId, email, or 'me'")
	return cmd
}
