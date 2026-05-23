package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func loginCmd() *cobra.Command {
	var email string
	var remember bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against Zelt (prompts for email, password, MFA code)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClientFromFlags(flagVerbose)
			if err != nil {
				return err
			}
			reader := bufio.NewReader(os.Stdin)

			if email == "" {
				fmt.Fprint(os.Stderr, "email: ")
				line, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				email = strings.TrimSpace(line)
			}
			if email == "" {
				return errors.New("email required")
			}

			fmt.Fprint(os.Stderr, "password: ")
			pwBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}
			password := string(pwBytes)
			if password == "" {
				return errors.New("password required")
			}

			prompt := func(method string) (string, error) {
				fmt.Fprintf(os.Stderr, "MFA code (%s): ", method)
				line, err := reader.ReadString('\n')
				return strings.TrimSpace(line), err
			}

			if err := c.passwordLogin(email, password, prompt); err != nil {
				return err
			}

			if remember {
				if err := c.store.SetPassword(email, password); err != nil {
					fmt.Fprintln(os.Stderr, "warning: could not save password to keychain:", err)
				} else {
					fmt.Fprintln(os.Stderr, "password saved to macOS Keychain (service=zeltapp-cli)")
				}
			}

			fmt.Fprintf(os.Stderr, "logged in as %s (userId=%d)\n", c.session.DisplayName, c.session.UserID)
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email (otherwise prompted)")
	cmd.Flags().BoolVar(&remember, "remember", true, "save password in macOS Keychain for auto re-login on session expiry (pass --remember=false to opt out)")
	return cmd
}

func logoutCmd() *cobra.Command {
	var forget bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear the saved session (use --forget to also delete the saved password)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := defaultStore()
			var email string
			if sess, err := s.LoadSession(); err == nil {
				email = sess.Email
			}
			if err := s.ClearSession(); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if forget && email != "" {
				if err := s.DeletePassword(email); err != nil {
					fmt.Fprintln(os.Stderr, "warning:", err)
				} else {
					fmt.Fprintln(os.Stderr, "removed saved password from keychain")
				}
			}
			fmt.Fprintln(os.Stderr, "logged out")
			return nil
		},
	}
	cmd.Flags().BoolVar(&forget, "forget", false, "also delete the saved password from macOS Keychain")
	return cmd
}

func whoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the currently authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(func(c *client) error {
				var out any
				if err := c.do("GET", "/apiv2/auth/me", nil, &out); err != nil {
					return err
				}
				if !flagJSON {
					fmt.Fprintf(os.Stderr, "%s <%s> userId=%d companyId=%d\n",
						c.session.DisplayName, c.session.Email, c.session.UserID, c.session.CompanyID)
				}
				return printResult(out)
			})
		},
	}
}

func mustURL(s string) *urlT { return parseURL(s) }
