package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagJSON    bool
	flagVerbose bool

	// version is injected at build time via -ldflags "-X main.version=...".
	version = "dev"

	// newClientHook is the factory used by withClient. Production points it at
	// newClientFromFlags; tests override it to inject a test client.
	newClientHook = func() (*client, error) { return newClientFromFlags(flagVerbose) }

	// outWriter is where renderJSON / renderRaw write. Tests redirect it.
	outWriter io.Writer = os.Stdout
)

func main() {
	root := &cobra.Command{
		Use:     "zeltapp",
		Short:   "Unofficial CLI for the Zelt HR platform",
		Long:    "zeltapp talks to https://go.zelt.app/apiv2/* using a cookie session.\nRun `zeltapp login` first.",
		Version: version,
		SilenceUsage:  true,
		SilenceErrors: true, // we render errors ourselves below
	}
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "JSON output instead of human-readable tables")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "print request/response to stderr")
	root.PersistentFlags().BoolVar(&flagNoCache, "no-cache", false, "bypass local TTL cache for cacheable endpoints")
	root.AddCommand(cacheCmd())

	root.AddCommand(loginCmd(), logoutCmd(), whoamiCmd())
	root.AddCommand(meCmd())
	root.AddCommand(peopleCmd())
	root.AddCommand(leaveCmd())
	root.AddCommand(attendanceCmd())
	root.AddCommand(calendarCmd())
	root.AddCommand(expensesCmd())
	root.AddCommand(companyCmd())
	root.AddCommand(reviewsCmd())
	root.AddCommand(goalsCmd())
	root.AddCommand(rawCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", errBody(err))
		os.Exit(1)
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

// printResult dispatches to JSON (--json) or human-readable output.
func printResult(v any) error {
	if flagJSON {
		return renderJSON(v)
	}
	return renderHuman(v)
}
