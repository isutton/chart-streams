package main

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const appName = "chart-streams"

// rootCmd main command.
var rootCmd = &cobra.Command{
	Use:   appName,
	Short: "Helm-Charts server backed by Git",
	Long: `## helmrepository

A Helm-Charts server using Git as a backend with semantic version support.

# Configuration

Command-line arguments can be expressed inline, or by exporting environment variables. For example,
the argument "--log-level" becomes "CHART_STREAMS_LOG_LEVEL". Note the prefix "CHART_STREAMS_" in
front of the actual argument, capitalization and substitution of dash ("-") by underscore ("_").

	`,
}

// init initialize the command-line flags and interpolation with environment.
func init() {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		logrus.SetOutput(os.Stdout)
		lvl, err := logrus.ParseLevel(viper.GetString("log-level"))
		if err != nil {
			return err
		}
		logrus.SetLevel(lvl)
		return nil
	}

	flags := rootCmd.PersistentFlags()

	// logrus log verbosity level
	flags.String("log-level", "info", "Log verbosity level (info, warn, error, debug)")

	// setting viper to search for environment variables based on application name and
	// parameter name joined together by underscore ("_"), and all capitalized.
	viper.SetEnvPrefix(appName)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	bindViperFlags(flags)
}

// bindViperFlags based on flag-set, creating a environment variable equivalent with Viper.
func bindViperFlags(flags *pflag.FlagSet) {
	if err := viper.BindPFlags(flags); err != nil {
		log.Fatal(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
