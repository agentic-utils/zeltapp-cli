package main

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func companyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "company",
		Aliases: []string{"org"},
		Short:   "Company config, departments, sites, job positions",
	}
	cmd.AddCommand(
		companyShowCmd(),
		companyDescribeCmd(),
		companyDepartmentsCmd(),
		companySitesCmd(),
		companyJobPositionsCmd(),
	)
	return cmd
}

func companyShowCmd() *cobra.Command {
	return rawJSONCmd("show", "Company config", "/apiv2/companies/config")
}

func companyDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe",
		Short: "Company config + general settings + departments + sites + job-positions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				out := map[string]json.RawMessage{}
				for label, path := range map[string]string{
					"config":          "/apiv2/companies/config",
					"generalSettings": "/apiv2/companies/general-settings",
					"departments":     "/apiv2/companies/departments",
					"sites":           "/apiv2/companies/sites",
					"jobPositions":    "/apiv2/job-positions",
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

func companyDepartmentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "department",
		Aliases: []string{"departments", "dept"},
		Short:   "Departments",
	}
	cmd.AddCommand(rawJSONCmd("list", "List departments", "/apiv2/companies/departments"))
	return cmd
}

func companySitesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "site",
		Aliases: []string{"sites"},
		Short:   "Sites",
	}
	cmd.AddCommand(rawJSONCmd("list", "List sites", "/apiv2/companies/sites"))
	return cmd
}

func companyJobPositionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "job-position",
		Aliases: []string{"job-positions", "job", "jobs"},
		Short:   "Job positions",
	}
	cmd.AddCommand(rawJSONCmd("list", "List job positions", "/apiv2/job-positions"))
	return cmd
}

func rawJSONCmd(use, short, path string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Aliases: []string{"ls"},
		Short:   short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				var v json.RawMessage
				if err := c.do("GET", path, nil, &v); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(v)})
			})
		},
	}
}
