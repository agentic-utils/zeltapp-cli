package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagJSON    bool // shorthand: -o json
	flagVerbose bool

	// version is injected at build time via -ldflags "-X main.version=...".
	version = "dev"

	// newClientHook is the factory used by withClient. Production points it at
	// newClientFromFlags; tests override it to inject a test client.
	newClientHook = func() (*client, error) { return newClientFromFlags(flagVerbose) }

	// outWriter / stderr / stdin are package-level IO so tests can redirect.
	outWriter io.Writer = os.Stdout
	stderr    io.Writer = os.Stderr
	stdin     io.Reader = os.Stdin
)

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, "error:", errBody(err))
		os.Exit(exitCodeFor(err))
	}
}

// exitCodeFor maps a top-level error to the documented CLI exit-code class.
//
//	0=success, 1=generic, 2=usage, 3=auth, 4=not-found,
//	5=rate-limited, 6=server, 7=network
//
// Only apiError carries enough information to differentiate auth/not-found/
// rate-limited/server; everything else collapses to 1.
func exitCodeFor(err error) int {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.ExitCode()
	}
	return 1
}

// newRootCmd wires resource-first commands together. Extracted so tests can
// build their own root without going through main().
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "zeltapp",
		Short:         "CLI for the Zelt HR platform",
		Long:          "zeltapp is a CLI for the Zelt HR platform.\nRun `zeltapp auth login` first.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&flagOutput, "output", "o", "table",
		"output format: table|json|yaml|wide|name")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "shorthand for -o json")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "log request/response to stderr")
	root.PersistentFlags().BoolVar(&flagNoCache, "no-cache", false, "bypass the local TTL cache")
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if flagJSON {
			flagOutput = outputJSON
		}
	}

	root.AddCommand(
		// auth + plumbing
		authCmd(),
		configCmd(),
		versionCmd(root),
		rawCmd(),
		// resources
		peopleCmd(),
		absenceCmd(),
		payslipCmd(),
		compensationCmd(),
		equityCmd(),
		pensionCmd(),
		benefitCmd(),
		contractCmd(),
		expenseCmd(),
		invoiceCmd(),
		deviceCmd(),
		attendanceCmd(),
		calendarCmd(),
		reviewCmd(),
		goalCmd(),
		companyCmd(),
	)
	return root
}

// versionCmd is inlined here (rather than its own file) since cobra's
// `--version` already prints `version` and the subcommand is a thin wrapper.
func versionCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the zeltapp version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(outWriter, version)
			return nil
		},
	}
}

// withClient runs f with an authenticated client. Use for commands that need a session.
func withClient(f func(c *client) error) error {
	c, err := newClientHook()
	if err != nil {
		return err
	}
	if err := c.requireSession(); err != nil {
		return err
	}
	return f(c)
}
