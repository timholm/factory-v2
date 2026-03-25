package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/timholm/factory-v2/internal/audit"
	"github.com/timholm/factory-v2/internal/build"
	"github.com/timholm/factory-v2/internal/config"
	"github.com/timholm/factory-v2/internal/db"
	"github.com/timholm/factory-v2/internal/discover"
	"github.com/timholm/factory-v2/internal/report"
	"github.com/timholm/factory-v2/internal/research"
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

			f := &Factory{
				cfg:        cfg,
				db:         store,
				discover:   discover.New(cfg.ArchiveURL),
				research:   research.New(cfg.GitHubToken),
				synthesize: synthesize.New(cfg.ClaudeBinary),
				build:      build.New(cfg, store),
				audit:      audit.New(cfg, store),
				report:     report.New(store),
			}

			return f.Run(ctx)
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

// Factory is the main orchestrator.
type Factory struct {
	cfg        *config.Config
	db         *db.Store
	discover   *discover.Discoverer
	research   *research.Researcher
	synthesize *synthesize.Synthesizer
	build      *build.Builder
	audit      *audit.Auditor
	report     *report.Reporter
}

// Run executes the full autonomous loop.
func (f *Factory) Run(ctx context.Context) error {
	for cycle := 1; ; cycle++ {
		log.Printf("[cycle %d] starting discovery", cycle)

		clusters, err := f.discover.FindClusters(ctx)
		if err != nil {
			log.Printf("[cycle %d] discover error: %v", cycle, err)
			goto sleep
		}

		log.Printf("[cycle %d] found %d clusters", cycle, len(clusters))

		for i, cluster := range clusters {
			if i >= f.cfg.MaxBuilds {
				break
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Persist cluster
			clusterID, err := f.db.InsertCluster(ctx, cluster)
			if err != nil {
				log.Printf("insert cluster: %v", err)
				continue
			}

			// Research
			log.Printf("[cluster %d] researching: %s", clusterID, cluster.ProblemSpace)
			if err := f.db.UpdateClusterStatus(ctx, clusterID, "researching"); err != nil {
				log.Printf("update status: %v", err)
			}

			res, err := f.research.Investigate(ctx, cluster)
			if err != nil {
				log.Printf("research error: %v", err)
				_ = f.db.UpdateClusterStatus(ctx, clusterID, "failed")
				continue
			}

			// Synthesize
			log.Printf("[cluster %d] synthesizing spec", clusterID)
			if err := f.db.UpdateClusterStatus(ctx, clusterID, "synthesizing"); err != nil {
				log.Printf("update status: %v", err)
			}

			spec, err := f.synthesize.Fuse(ctx, res)
			if err != nil {
				log.Printf("synthesize error: %v", err)
				_ = f.db.UpdateClusterStatus(ctx, clusterID, "failed")
				continue
			}

			// Save spec
			if err := f.db.SaveSpec(ctx, clusterID, spec); err != nil {
				log.Printf("save spec: %v", err)
			}
			if err := f.db.UpdateClusterStatus(ctx, clusterID, "building"); err != nil {
				log.Printf("update status: %v", err)
			}

			// Build
			log.Printf("[cluster %d] building: %s", clusterID, spec.Name)
			buildID, err := f.db.InsertBuild(ctx, clusterID, spec)
			if err != nil {
				log.Printf("insert build: %v", err)
				continue
			}

			if err := f.build.Execute(ctx, spec); err != nil {
				log.Printf("build error: %v", err)
				_ = f.db.UpdateBuildStatus(ctx, buildID, "failed", err.Error())
				_ = f.db.UpdateClusterStatus(ctx, clusterID, "failed")
				continue
			}

			githubURL := fmt.Sprintf("https://github.com/%s/%s", f.cfg.GitHubUser, spec.Name)
			_ = f.db.UpdateBuildShipped(ctx, buildID, githubURL)
			_ = f.db.UpdateClusterStatus(ctx, clusterID, "shipped")

			// Audit
			score, err := f.audit.Score(ctx, spec.Name)
			if err != nil {
				log.Printf("audit error: %v", err)
			} else {
				_ = f.db.UpdateBuildScore(ctx, buildID, score)
				if score < 50 {
					log.Printf("[cluster %d] score %d < 50, deleting repo %s", clusterID, score, spec.Name)
					_ = f.audit.Delete(ctx, spec.Name)
					_ = f.db.UpdateBuildStatus(ctx, buildID, "failed", "quality score too low")
				}
			}
		}

		// Report
		if err := f.report.Generate(ctx); err != nil {
			log.Printf("[cycle %d] report error: %v", cycle, err)
		}

	sleep:
		log.Printf("[cycle %d] sleeping %s", cycle, f.cfg.CycleInterval)
		timer := time.NewTimer(f.cfg.CycleInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
