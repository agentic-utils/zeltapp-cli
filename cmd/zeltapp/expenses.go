package main

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func expensesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expenses",
		Short: "Expense claims",
	}
	cmd.AddCommand(expensesListCmd(), invoicesListCmd())
	return cmd
}

func expensesListCmd() *cobra.Command {
	var pageSize int
	var filter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List expense claims",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				q := url.Values{}
				q.Set("page", "1")
				q.Set("pageSize", fmt.Sprintf("%d", pageSize))
				if filter != "" {
					q.Set("filters", filter)
				}
				var out json.RawMessage
				if err := c.do("GET", "/apiv2/expense/user/paginated?"+q.Encode(), nil, &out); err != nil {
					return err
				}
				return renderRaw(out)
			})
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "page size")
	cmd.Flags().StringVar(&filter, "filter", "", "raw filters query string")
	return cmd
}

func invoicesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "invoices",
		Short: "List contractor invoices for the current user",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				p := fmt.Sprintf("/apiv2/contractor/invoice/users/%d", c.session.UserID)
				var out json.RawMessage
				if err := c.do("GET", p, nil, &out); err != nil {
					return err
				}
				return renderRaw(out)
			})
		},
	}
}
