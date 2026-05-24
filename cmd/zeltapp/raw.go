package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// validMethods are the HTTP verbs the raw escape hatch will accept. Anything
// else gets a usage error before we even build a request.
var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "HEAD": true,
}

func rawCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "raw METHOD PATH [JSON_BODY]",
		Short: "Escape hatch: call any /apiv2/* endpoint (auth endpoints blocked)",
		Long: `Examples:
  zeltapp raw GET  /apiv2/users/cache
  zeltapp raw POST /apiv2/absences/verify-overlap '{"absenceStart":"2026-06-01","userIds":[6380]}'

For safety, raw refuses paths outside /apiv2/ and refuses any /apiv2/auth/*
endpoint (use 'zeltapp auth login' / 'auth logout' instead). This stops an
attacker who can run one command in your shell from rebinding your persisted
session via 'raw POST /apiv2/auth/login'.`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			if !validMethods[method] {
				return fmt.Errorf("unknown HTTP method %q (want one of GET/POST/PUT/PATCH/DELETE/HEAD)", method)
			}
			path := args[1]
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			if !strings.HasPrefix(path, "/apiv2/") {
				return fmt.Errorf("raw refuses paths outside /apiv2/ (got %q)", path)
			}
			if strings.HasPrefix(path, "/apiv2/auth/") {
				return fmt.Errorf("raw refuses /apiv2/auth/* — use `zeltapp auth login` or `zeltapp auth logout`")
			}
			var body any
			if len(args) == 3 {
				if method == "GET" || method == "HEAD" {
					return fmt.Errorf("%s requests do not accept a body", method)
				}
				if err := json.Unmarshal([]byte(args[2]), &body); err != nil {
					return errors.New("body must be valid JSON: " + err.Error())
				}
			}
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				var out json.RawMessage
				if err := c.do(method, path, body, &out); err != nil {
					return err
				}
				return emit(&resourceView{raw: rawToAny(out)})
			})
		},
	}
}
