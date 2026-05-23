package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func meCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "me",
		Short: "Read your own profile, employment, finance and IT records",
	}
	cmd.AddCommand(
		meProfileCmd(),
		meContactCmd(),
		meEmploymentCmd(),
		meCompensationCmd(),
		meBankCmd(),
		meEquityCmd(),
		mePensionCmd(),
		mePayslipsCmd(),
		meDevicesCmd(),
		meBenefitsCmd(),
	)
	return cmd
}

// fetchMany hits N endpoints under /apiv2/users/<self>/... and returns a map of {path: result}.
func fetchMany(c *client, paths ...string) (map[string]json.RawMessage, error) {
	return fetchManyForUser(c, c.session.UserID, paths...)
}

// fetchManyForUser is the same as fetchMany but for any user (used by `people get`).
func fetchManyForUser(c *client, userID int, paths ...string) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	for _, p := range paths {
		var v json.RawMessage
		full := fmt.Sprintf("/apiv2/users/%d/%s", userID, p)
		if err := c.do("GET", full, nil, &v); err != nil {
			return nil, err
		}
		out[p] = v
	}
	return out, nil
}

func meProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "profile",
		Short: "basic + personal + about + missing fields",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				out, err := fetchMany(c, "basic", "personal", "about", "missing-fields")
				if err != nil {
					return err
				}
				return printResult(out)
			})
		},
	}
}

func meContactCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "contact",
		Short: "address + emergency contact + work contact + family",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				out, err := fetchMany(c, "address", "emergency-contact", "work-contact", "family", "family/members")
				if err != nil {
					return err
				}
				return printResult(out)
			})
		},
	}
}

func meEmploymentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "employment",
		Short: "role + contracts + lifecycle + right-to-work + summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				out, err := fetchMany(c, "role", "contracts", "contracts/current", "lifecycle", "events",
					"right-work/documents", "summary?reports=true")
				if err != nil {
					return err
				}
				return printResult(out)
			})
		},
	}
}

func meCompensationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compensation",
		Short: "compensation history",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				out, err := fetchMany(c, "compensation")
				if err != nil {
					return err
				}
				return printResult(out)
			})
		},
	}
}

func meBankCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bank",
		Short: "bank accounts on file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				out, err := fetchMany(c, "bank-accounts")
				if err != nil {
					return err
				}
				return printResult(out)
			})
		},
	}
}

func meEquityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "equity",
		Short: "equity grants",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				out, err := fetchMany(c, "equity")
				if err != nil {
					return err
				}
				return printResult(out)
			})
		},
	}
}

func mePensionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pension",
		Short: "pension scheme + contributions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				out := map[string]json.RawMessage{}
				for _, suffix := range []string{"", "/contributions"} {
					var v json.RawMessage
					p := fmt.Sprintf("/apiv2/employees/%d/pension%s", c.session.UserID, suffix)
					if err := c.do("GET", p, nil, &v); err != nil {
						return err
					}
					if suffix == "" {
						out["pension"] = v
					} else {
						out["contributions"] = v
					}
				}
				return printResult(out)
			})
		},
	}
}

func mePayslipsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "payslips",
		Short: "payrolls + payslips list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				out, err := fetchMany(c, "payrolls", "payslips")
				if err != nil {
					return err
				}
				return printResult(out)
			})
		},
	}
}

func meDevicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "devices",
		Short: "IT devices assigned to you (+ orders, in-transit)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				out := map[string]json.RawMessage{}
				for _, p := range []string{"devices/users", "devices/users/%d/in-transit", "devices/orders/users"} {
					var v json.RawMessage
					var path string
					if p == "devices/users/%d/in-transit" {
						path = fmt.Sprintf("/apiv2/devices/users/%d/in-transit", c.session.UserID)
					} else {
						path = fmt.Sprintf("/apiv2/%s/%d", p, c.session.UserID)
					}
					if err := c.do("GET", path, nil, &v); err != nil {
						return err
					}
					out[path] = v
				}
				return printResult(out)
			})
		},
	}
}

func meBenefitsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "benefits",
		Short: "active custom benefits",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				var v json.RawMessage
				p := fmt.Sprintf("/apiv2/custom-benefit/by-user/%d/effective", c.session.UserID)
				if err := c.do("GET", p, nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	}
}
