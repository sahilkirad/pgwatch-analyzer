package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "pgwatch-ai",
	Short: "AI-assisted PostgreSQL diagnostics for pgwatch metrics",
	Long:  "pgwatch-ai analyzes pgwatch sink metrics and provides diagnostic insights.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(newAskCmd())
}
