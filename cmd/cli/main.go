package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sentioxyz/changelogue/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var version = "dev"

// errAlreadyPrinted wraps an error that has already been displayed to the user.
type errAlreadyPrinted struct{ err error }

func (e errAlreadyPrinted) Error() string { return e.err.Error() }

func main() {
	if err := rootCmd.Execute(); err != nil {
		if _, ok := err.(errAlreadyPrinted); !ok {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

var (
	serverURL string
	apiKey    string
	jsonOut   bool
)

var rootCmd = &cobra.Command{
	Use:   "clog",
	Short: "Changelogue CLI — manage projects, sources, releases, channels, and subscriptions",
	Long: `clog is the command-line interface for Changelogue.

It talks to a running Changelogue server via its REST API.
Configure the server URL and API key via flags or environment variables:

  export CHANGELOGUE_SERVER=http://localhost:8080
  export CHANGELOGUE_API_KEY=rg_live_abc123...

Examples:
  clog projects list
  clog sources create --project <id> --provider dockerhub --repository library/postgres
  clog releases list --project <id>
  clog channels create --name my-slack --type slack --config '{"webhook_url":"https://..."}'`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("clog version", version)
	},
}

// newClient builds a Client from resolved global flags. Called at command execution
// time (not init time) so that flag values and env vars are available.
func newClient() *cli.Client {
	return cli.NewClient(resolveServerURL(), resolveAPIKey())
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "Changelogue server URL (env: CHANGELOGUE_SERVER)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication (env: CHANGELOGUE_API_KEY)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output raw JSON instead of table")

	rootCmd.AddCommand(versionCmd)

	// Resource subcommands — each takes newClient so the client is built lazily.
	rootCmd.AddCommand(cli.NewProjectsCmd(newClient, &jsonOut))
	rootCmd.AddCommand(cli.NewSourcesCmd(newClient, &jsonOut))
	rootCmd.AddCommand(cli.NewReleasesCmd(newClient, &jsonOut))
	rootCmd.AddCommand(cli.NewChannelsCmd(newClient, &jsonOut))
	rootCmd.AddCommand(cli.NewSubscriptionsCmd(newClient, &jsonOut))

	// AI-friendly hints: suggest commands on typo
	rootCmd.SuggestionsMinimumDistance = 2

	// Custom error formatting for unknown flags
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		msg := err.Error()
		if strings.Contains(msg, "unknown flag") {
			cmd.PrintErrln("Error:", msg)
			cmd.PrintErrln()
			cmd.PrintErrln("Available flags:")
			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				cmd.PrintErrf("  --%s\t%s\n", f.Name, f.Usage)
			})
			return errAlreadyPrinted{err}
		}
		return err
	})
}

func resolveServerURL() string {
	if serverURL != "" {
		return serverURL
	}
	if v := os.Getenv("CHANGELOGUE_SERVER"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

func resolveAPIKey() string {
	if apiKey != "" {
		return apiKey
	}
	return os.Getenv("CHANGELOGUE_API_KEY")
}
