package main

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate against Zelt",
	}
	cmd.AddCommand(loginCmd(), logoutCmd(), whoamiCmd())
	return cmd
}

func loginCmd() *cobra.Command {
	var email string
	var remember bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate (prompts for email, password, MFA code)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClientFromFlags(flagVerbose)
			if err != nil {
				return err
			}
			reader := bufio.NewReader(stdin)

			if email == "" {
				fmt.Fprint(stderr, "email: ")
				line, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				email = strings.TrimSpace(line)
			}
			if email == "" {
				return errors.New("email required")
			}

			fmt.Fprint(stderr, "password: ")
			pwBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(stderr)
			if err != nil {
				return err
			}
			password := string(pwBytes)
			if password == "" {
				return errors.New("password required")
			}

			prompt := func(method string) (string, error) {
				fmt.Fprintf(stderr, "MFA code (%s): ", method)
				line, err := reader.ReadString('\n')
				return strings.TrimSpace(line), err
			}

			if err := c.passwordLogin(email, password, prompt); err != nil {
				return err
			}

			if remember {
				if err := c.store.SetPassword(email, password); err != nil {
					// Fail loud rather than silently degrading. INF-1314 calls
					// this out explicitly: on a Linux / headless box without a
					// keychain backend, the user must consciously pass
					// --remember=false rather than discover months later that
					// auto-relogin never worked.
					return fmt.Errorf("could not save password to keychain: %w\n  pass --remember=false to log in without persisting", err)
				}
				fmt.Fprintln(stderr, "password saved to macOS Keychain (service=zeltapp-cli)")
			}

			fmt.Fprintf(stderr, "logged in as %s (userId=%d)\n", c.session.DisplayName, c.session.UserID)
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email (otherwise prompted)")
	cmd.Flags().BoolVar(&remember, "remember", true, "save password in macOS Keychain (pass --remember=false to opt out)")
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
			if err := s.ClearSession(); err != nil {
				return err
			}
			// Clear the on-disk cache too — it may contain the previous
			// user's company directory / PII (review #9). Not fatal if it
			// fails (e.g. cache dir doesn't exist).
			if err := cacheClear(); err != nil {
				fmt.Fprintln(stderr, "warning: could not clear cache:", err)
			}
			if forget && email != "" {
				if err := s.DeletePassword(email); err != nil {
					fmt.Fprintln(stderr, "warning:", err)
				} else {
					fmt.Fprintln(stderr, "removed saved password from keychain")
				}
			}
			fmt.Fprintln(stderr, "logged out")
			return nil
		},
	}
	cmd.Flags().BoolVar(&forget, "forget", false, "also delete saved password from macOS Keychain")
	return cmd
}

func whoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the currently authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				var out any
				if err := c.do("GET", "/apiv2/auth/me", nil, &out); err != nil {
					return err
				}
				return emit(&resourceView{raw: out})
			})
		},
	}
}

