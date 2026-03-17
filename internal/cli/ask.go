package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"pgwatchai/internal/app"

	"github.com/spf13/cobra"
)

func newAskCmd() *cobra.Command {
	var sinkDSN string

	cmd := &cobra.Command{
		Use:   "ask \"your question\"",
		Short: "Ask pgwatch-ai to analyze pgwatch metrics using natural language",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.TrimSpace(strings.Join(args, " "))
			if prompt == "" {
				return errors.New("prompt cannot be empty")
			}

			if strings.TrimSpace(sinkDSN) == "" {
				sinkDSN = strings.TrimSpace(os.Getenv("PWAI_SINK_DSN"))
			}
			if sinkDSN == "" {
				return errors.New("sink dsn is required: use --sink-dsn or PWAI_SINK_DSN")
			}

			orch := app.NewOrchestrator()
			resp, err := orch.Handle(prompt, sinkDSN)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), resp)
			return nil
		},
	}

	cmd.Flags().StringVar(&sinkDSN, "sink-dsn", "", "PostgreSQL DSN for pgwatch sink database")
	return cmd
}
