package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X agent_stock/cmd.Version=v0.1.0"
var Version = "dev"

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "agent",
	Short: "AI agent gateway (stock research) — backend only",
	Long:  "Phase-by-phase Go rewrite inspired by goclaw + ews-agent. No web UI.",
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")

	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(skillsCmd())
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("agent %s\n", Version)
		},
	}
}

// Execute runs the root cobra command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
