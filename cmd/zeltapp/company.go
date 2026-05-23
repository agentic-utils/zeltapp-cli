package main

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func companyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "company",
		Short: "Read company / org configuration",
	}
	cmd.AddCommand(
		simpleGet("config", "Company config", "/apiv2/companies/config"),
		simpleGet("general-settings", "Company general settings", "/apiv2/companies/general-settings"),
		simpleGet("departments", "Departments", "/apiv2/companies/departments"),
		simpleGet("sites", "Sites", "/apiv2/companies/sites"),
		simpleGet("job-positions", "Job positions", "/apiv2/job-positions"),
		simpleGet("forms", "Forms", "/apiv2/company/forms"),
		simpleGet("fields", "Profile fields", "/apiv2/company/fields/all-fields-profile"),
		simpleGet("reports", "Reports list", "/apiv2/reports/all/new"),
		simpleGet("apps", "Installed apps", "/apiv2/apps"),
		simpleGet("apps-install", "Apps available to install", "/apiv2/apps/install"),
	)
	return cmd
}

func simpleGet(use, short, path string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", path, nil, &v); err != nil {
					return err
				}
				return renderRaw(v)
			})
		},
	}
}
