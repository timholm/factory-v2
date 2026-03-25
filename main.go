package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/timholm/factory-v2/internal/audit"
	"github.com/timholm/factory-v2/internal/build"
	"github.com/timholm/factory-v2/internal/config"
	"github.com/timholm/factory-v2/internal/db"
	"github.com/timholm/factory-v2/internal/report"
	"github.com/timholm/factory-v2/internal/synthesize"
)

func main() {
	root := &cobra.Command{
		Use:   "factory-v2",
		Short: "Autonomous research-to-product factory",
		Long:  "One binary, one loop. Discovers arXiv papers, clusters them, researches techniques, synthesizes fusion specs, builds repos, validates, and reports.",
	}

	root.AddCommand(runCmd())
	root.AddCommand(buildCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(auditCmd())
	root.AddCommand(overseerCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Full autonomous loop: discover -> research -> synthesize -> build -> validate -> report",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			store, err := db.New(ctx, cfg.PostgresURL)
			if err != nil {
				return fmt.Errorf("db connect: %w", err)
			}
			defer store.Close()

			if err := store.Migrate(ctx); err != nil {
				return fmt.Errorf("db migrate: %w", err)
			}

			// v3 pipeline: streaming, auto-scaling, self-healing
			p := NewPipeline(cfg, store)
			return p.Run(ctx)
		},
	}
}

func buildCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build one repo from a spec file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			ctx := context.Background()

			store, err := db.New(ctx, cfg.PostgresURL)
			if err != nil {
				return fmt.Errorf("db connect: %w", err)
			}
			defer store.Close()

			if err := store.Migrate(ctx); err != nil {
				return fmt.Errorf("db migrate: %w", err)
			}

			spec, err := synthesize.LoadSpec(specFile)
			if err != nil {
				return fmt.Errorf("load spec: %w", err)
			}

			builder := build.New(cfg, store)
			return builder.Execute(ctx, spec)
		},
	}
	cmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to spec JSON file")
	_ = cmd.MarkFlagRequired("spec")
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show pipeline status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			ctx := context.Background()

			store, err := db.New(ctx, cfg.PostgresURL)
			if err != nil {
				return fmt.Errorf("db connect: %w", err)
			}
			defer store.Close()

			return report.PrintStatus(ctx, store)
		},
	}
}

func auditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit",
		Short: "Run quality audit on all shipped repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			ctx := context.Background()

			store, err := db.New(ctx, cfg.PostgresURL)
			if err != nil {
				return fmt.Errorf("db connect: %w", err)
			}
			defer store.Close()

			a := audit.New(cfg, store)
			return a.RunAll(ctx)
		},
	}
}

func overseerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "overseer",
		Short: "Run one overseer audit — Opus critiques the factory like Tim would",
		RunE: func(cmd *cobra.Command, args []string) error {
			runOverseer()
			return nil
		},
	}
}

