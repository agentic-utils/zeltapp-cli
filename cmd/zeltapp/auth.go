package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

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
	var passwordStdin bool
	var mfaCommand string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate (prompts for email, password, MFA code)",
		Long: "Authenticate against Zelt.\n\n" +
			"Interactive by default (prompts for email, password, MFA code). For headless\n" +
			"or scripted use, pass --password-stdin to read the password from stdin and\n" +
			"--mfa-command to fetch the emailed MFA code without a prompt:\n\n" +
			"  your-password-source | \\\n" +
			"    zeltapp auth login -e me@example.com --password-stdin \\\n" +
			"      --mfa-command 'fetch-code.sh'\n\n" +
			"--mfa-command runs after the code is sent; its stdout must contain the\n" +
			"6-digit code (any surrounding text is ignored). ZELT_MFA_METHOD and\n" +
			"ZELT_MFA_SINCE (unix seconds, set just before the send) are exported to it.",
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

			var password string
			if passwordStdin {
				line, err := reader.ReadString('\n')
				if err != nil && line == "" {
					return fmt.Errorf("reading password from stdin: %w", err)
				}
				password = strings.TrimRight(line, "\r\n")
			} else {
				fmt.Fprint(stderr, "password: ")
				pwBytes, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Fprintln(stderr)
				if err != nil {
					return err
				}
				password = string(pwBytes)
			}
			if password == "" {
				return errors.New("password required")
			}

			prompt := func(method string) (string, error) {
				if mfaCommand != "" {
					return runMFACommand(mfaCommand, method)
				}
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
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read password from stdin instead of a TTY prompt")
	cmd.Flags().StringVar(&mfaCommand, "mfa-command", "", "shell command whose stdout yields the MFA code (no TTY prompt)")
	return cmd
}

// runMFACommand runs the --mfa-command via `sh -c` and extracts a 6-digit code
// from its stdout. It runs after the code has been sent, so a command that
// polls email finds a fresh one. ZELT_MFA_METHOD and ZELT_MFA_SINCE (unix
// seconds, sampled just before this call) are exported so the command can
// ignore codes minted before this login attempt.
func runMFACommand(command, method string) (string, error) {
	c := exec.Command("sh", "-c", command)
	c.Env = append(os.Environ(),
		"ZELT_MFA_METHOD="+method,
		fmt.Sprintf("ZELT_MFA_SINCE=%d", time.Now().Unix()),
	)
	c.Stderr = stderr
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("mfa-command failed: %w", err)
	}
	m := regexp.MustCompile(`\d{6}`).FindString(string(out))
	if m == "" {
		return "", errors.New("mfa-command produced no 6-digit code")
	}
	return m, nil
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
