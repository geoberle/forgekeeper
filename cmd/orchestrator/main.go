package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/geoberle/software-factory/internal/orchestrator"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := funcr.New(func(prefix, args string) { fmt.Fprintln(os.Stderr, prefix+args) }, funcr.Options{})
	raw := orchestrator.DefaultOptions()
	cmd := &cobra.Command{Use: "orchestrator", Short: "Poll GitHub issues and coordinate Fabro runs", SilenceErrors: true, SilenceUsage: true, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			valid, err := raw.Validate()
			if err != nil {
				logger.Error(err, "Configuration failed")
				return err
			}
			complete, err := valid.Complete(ctx, logger)
			if err != nil {
				logger.Error(err, "Configuration failed")
				return err
			}
			return complete.Run(ctx)
		}}
	orchestrator.BindOptions(raw, cmd)
	if err := cmd.ExecuteContext(ctx); err != nil {
		// Cobra flag errors may echo user input. Keep the top-level error generic.
		logger.Info("Orchestrator stopped; check configuration and the preceding safe diagnostics")
		os.Exit(1)
	}
}
