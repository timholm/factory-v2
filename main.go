package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
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

func (f *Factory) sleepCycle(ctx context.Context, cycle int) {
	log.Printf("[cycle %d] sleeping %s", cycle, f.cfg.CycleInterval)
	timer := time.NewTimer(f.cfg.CycleInterval)
	select {
	case <-ctx.Done():
		timer.Stop()
	case <-timer.C:
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
			f.sleepCycle(ctx, cycle)
			continue
		}

		log.Printf("[cycle %d] found %d clusters", cycle, len(clusters))

		// Research + synthesize all clusters first (fast, uses Opus)
		type buildJob struct {
			clusterID int
			spec      *synthesize.ProductSpec
		}
		var jobs []buildJob

		for i, cluster := range clusters {
			if i >= f.cfg.MaxBuilds {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			clusterID, err := f.db.InsertCluster(ctx, cluster)
			if err != nil {
				log.Printf("insert cluster: %v", err)
				continue
			}

			log.Printf("[cluster %d] researching: %s", clusterID, cluster.ProblemSpace)
			_ = f.db.UpdateClusterStatus(ctx, clusterID, "researching")

			res, err := f.research.Investigate(ctx, cluster)
			if err != nil {
				log.Printf("research error: %v", err)
				_ = f.db.UpdateClusterStatus(ctx, clusterID, "failed")
				continue
			}

			log.Printf("[cluster %d] synthesizing spec", clusterID)
			_ = f.db.UpdateClusterStatus(ctx, clusterID, "synthesizing")

			spec, err := f.synthesize.Fuse(ctx, res)
			if err != nil {
				log.Printf("synthesize error: %v", err)
				_ = f.db.UpdateClusterStatus(ctx, clusterID, "failed")
				continue
			}

			_ = f.db.SaveSpec(ctx, clusterID, spec)
			_ = f.db.UpdateClusterStatus(ctx, clusterID, "building")
			jobs = append(jobs, buildJob{clusterID: clusterID, spec: spec})
		}

		// Build in parallel — 6 workers, each gets its own tmux session
		workers := 6
		if len(jobs) < workers {
			workers = len(jobs)
		}
		log.Printf("[cycle %d] launching %d parallel builds (%d jobs)", cycle, workers, len(jobs))

		jobCh := make(chan buildJob, len(jobs))
		for _, j := range jobs {
			jobCh <- j
		}
		close(jobCh)

		var wg sync.WaitGroup
		var mirrorMu sync.Mutex // serialize GitHub pushes

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for job := range jobCh {
					spec := job.spec
					clusterID := job.clusterID

					log.Printf("[worker %d] building: %s", workerID, spec.Name)
					buildID, err := f.db.InsertBuild(ctx, clusterID, spec)
					if err != nil {
						log.Printf("[worker %d] insert build: %v", workerID, err)
						continue
					}

					if err := f.build.Execute(ctx, spec); err != nil {
						log.Printf("[worker %d] build failed: %s — %v", workerID, spec.Name, err)
						_ = f.db.UpdateBuildStatus(ctx, buildID, "failed", err.Error())
						_ = f.db.UpdateClusterStatus(ctx, clusterID, "failed")
						continue
					}

					// Serialize GitHub push (only one at a time)
					mirrorMu.Lock()
					githubURL := fmt.Sprintf("https://github.com/%s/%s", f.cfg.GitHubUser, spec.Name)
					_ = f.db.UpdateBuildShipped(ctx, buildID, githubURL)
					_ = f.db.UpdateClusterStatus(ctx, clusterID, "shipped")
					mirrorMu.Unlock()

					log.Printf("[worker %d] shipped: %s", workerID, spec.Name)

					// Audit
					score, err := f.audit.Score(ctx, spec.Name)
					if err != nil {
						log.Printf("[worker %d] audit error: %v", workerID, err)
					} else {
						_ = f.db.UpdateBuildScore(ctx, buildID, score)
						if score < 50 {
							log.Printf("[worker %d] score %d < 50, deleting %s", workerID, score, spec.Name)
							_ = f.audit.Delete(ctx, spec.Name)
							_ = f.db.UpdateBuildStatus(ctx, buildID, "failed", "quality score too low")
						}
					}
				}
			}(w)
		}
		wg.Wait()

		// Report
		if err := f.report.Generate(ctx); err != nil {
			log.Printf("[cycle %d] report error: %v", cycle, err)
		}

		f.sleepCycle(ctx, cycle)
	}
}
