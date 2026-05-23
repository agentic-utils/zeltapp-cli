package main

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

func rawCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "raw METHOD PATH [JSON_BODY]",
		Short: "Escape hatch: call any /apiv2/* endpoint",
		Long: `Examples:
  zeltapp raw GET  /apiv2/users/cache
  zeltapp raw POST /apiv2/absences/verify-overlap '{"absenceStart":"2026-06-01","userIds":[6380]}'`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			var body any
			if len(args) == 3 {
				if err := json.Unmarshal([]byte(args[2]), &body); err != nil {
					return errors.New("body must be valid JSON: " + err.Error())
				}
			}
			return withClient(func(c *client) error {
				var out json.RawMessage
				if err := c.do(method, path, body, &out); err != nil {
					return err
				}
				return renderRaw(out)
			})
		},
	}
}
