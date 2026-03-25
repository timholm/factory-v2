package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/timholm/factory-v2/internal/audit"
	"github.com/timholm/factory-v2/internal/build"
	"github.com/timholm/factory-v2/internal/config"
	"github.com/timholm/factory-v2/internal/db"
	"github.com/timholm/factory-v2/internal/discover"
	"github.com/timholm/factory-v2/internal/report"
	"github.com/timholm/factory-v2/internal/research"
	"github.com/timholm/factory-v2/internal/synthesize"
)

// Pipeline is the v3 architecture: streaming pipeline with auto-scaling workers.
type Pipeline struct {
	cfg        *config.Config
	db         *db.Store
	discover   *discover.Discoverer
	research   *research.Researcher
	synthesize *synthesize.Synthesizer
	build      *build.Builder
	audit      *audit.Auditor
	report     *report.Reporter

	specCh     chan *synthesize.ProductSpec
	workers    int32 // current worker count (atomic)
	maxWorkers int32
	rateLimited int32 // 1 if rate limited, 0 if not
}

func NewPipeline(cfg *config.Config, store *db.Store) *Pipeline {
	return &Pipeline{
		cfg:        cfg,
		db:         store,
		discover:   discover.New(cfg.ArchiveURL),
		research:   research.New(cfg.GitHubToken),
		synthesize: synthesize.New(cfg.ClaudeBinary),
		build:      build.New(cfg, store),
		audit:      audit.New(cfg, store),
		report:     report.New(store),
		specCh:     make(chan *synthesize.ProductSpec, 20),
		maxWorkers: 4,
	}
}

// Run starts the v3 pipeline: discoverer + builder pool + overseer + healer.
func (p *Pipeline) Run(ctx context.Context) error {
	log.Println("[v3] starting pipeline — discoverer + builders + overseer + healer")

	var wg sync.WaitGroup

	// Discoverer: continuously finds papers, researches, synthesizes, pushes specs
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.runDiscoverer(ctx)
	}()

	// Builder pool: starts at 2, scales up to 6
	atomic.StoreInt32(&p.workers, 0)
	initialWorkers := int32(4)
	for i := int32(0); i < initialWorkers; i++ {
		p.spawnWorker(ctx, &wg, i)
	}

	// Overseer: continuous critique
	wg.Add(1)
	go func() {
		defer wg.Done()
		RunOverseerLoop(ctx)
	}()

	// Healer: self-repair every 60s
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.runHealer(ctx)
	}()

	// Auto-scaler: watches queue depth and adjusts workers
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.runAutoScaler(ctx, &wg)
	}()

	// Report every 10 minutes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Minute):
				p.report.Generate(ctx)
			}
		}
	}()

	wg.Wait()
	return ctx.Err()
}

func (p *Pipeline) spawnWorker(ctx context.Context, wg *sync.WaitGroup, id int32) {
	atomic.AddInt32(&p.workers, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer atomic.AddInt32(&p.workers, -1)
		p.runBuilder(ctx, id)
	}()
	log.Printf("[v3] spawned worker %d (total: %d)", id, atomic.LoadInt32(&p.workers))
}

// runDiscoverer continuously discovers and synthesizes specs.
func (p *Pipeline) runDiscoverer(ctx context.Context) {
	cycle := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cycle++
		log.Printf("[discover] cycle %d starting", cycle)

		clusters, err := p.discover.FindClusters(ctx)
		if err != nil {
			log.Printf("[discover] error: %v", err)
			time.Sleep(5 * time.Minute)
			continue
		}

		log.Printf("[discover] found %d clusters", len(clusters))

		for i, cluster := range clusters {
			if i >= p.cfg.MaxBuilds {
				break
			}

			select {
			case <-ctx.Done():
				return
			default:
			}

			// Research
			clusterID, _ := p.db.InsertCluster(ctx, cluster)
			log.Printf("[research] cluster %d: %s", clusterID, cluster.ProblemSpace)
			p.db.UpdateClusterStatus(ctx, clusterID, "researching")

			res, err := p.research.Investigate(ctx, cluster)
			if err != nil {
				log.Printf("[research] error: %v", err)
				p.db.UpdateClusterStatus(ctx, clusterID, "failed")
				continue
			}

			// Synthesize — immediately push to builders
			log.Printf("[synthesize] cluster %d", clusterID)
			p.db.UpdateClusterStatus(ctx, clusterID, "synthesizing")

			spec, err := p.synthesize.Fuse(ctx, res)
			if err != nil {
				log.Printf("[synthesize] error: %v", err)
				p.db.UpdateClusterStatus(ctx, clusterID, "failed")
				continue
			}

			p.db.SaveSpec(ctx, clusterID, spec)
			p.db.UpdateClusterStatus(ctx, clusterID, "queued")

			log.Printf("[pipeline] queued %s for build", spec.Name)
			p.specCh <- spec
		}
	}
}

// runBuilder pulls specs and builds them. Handles rate limits gracefully.
func (p *Pipeline) runBuilder(ctx context.Context, id int32) {
	for {
		select {
		case <-ctx.Done():
			return
		case spec := <-p.specCh:
			log.Printf("[worker %d] building %s", id, spec.Name)

			buildID, _ := p.db.InsertBuild(ctx, 0, spec)

			err := p.build.Execute(ctx, spec)
			if err != nil {
				errStr := err.Error()
				log.Printf("[worker %d] build failed: %s — %v", id, spec.Name, err)
				p.db.UpdateBuildStatus(ctx, buildID, "failed", errStr)

				// Rate limit detection — pause and resume
				if isRateLimit(errStr) {
					resetTime := parseResetTime(errStr)
					log.Printf("[worker %d] RATE LIMITED — waiting until %s", id, resetTime.Format("15:04:05"))
					atomic.StoreInt32(&p.rateLimited, 1)

					waitDuration := time.Until(resetTime) + time.Minute
					if waitDuration < time.Minute {
						waitDuration = 5 * time.Minute
					}
					if waitDuration > 2 * time.Hour {
						waitDuration = 2 * time.Hour
					}

					select {
					case <-ctx.Done():
						return
					case <-time.After(waitDuration):
					}

					atomic.StoreInt32(&p.rateLimited, 0)
					log.Printf("[worker %d] rate limit cleared, resuming", id)

					// Re-queue the spec
					select {
					case p.specCh <- spec:
						log.Printf("[worker %d] re-queued %s", id, spec.Name)
					default:
					}
					continue
				}
				continue
			}

			githubURL := fmt.Sprintf("https://github.com/%s/%s", p.cfg.GitHubUser, spec.Name)
			p.db.UpdateBuildShipped(ctx, buildID, githubURL)
			log.Printf("[worker %d] SHIPPED %s → %s", id, spec.Name, githubURL)

			// Audit
			score, err := p.audit.Score(ctx, spec.Name)
			if err == nil {
				p.db.UpdateBuildScore(ctx, buildID, score)
				if score < 50 {
					log.Printf("[worker %d] score %d < 50, deleting %s", id, score, spec.Name)
					p.audit.Delete(ctx, spec.Name)
				}
			}
		}
	}
}

// runAutoScaler adjusts worker count based on queue depth and rate limits.
func (p *Pipeline) runAutoScaler(ctx context.Context, wg *sync.WaitGroup) {
	nextID := int32(2) // workers 0,1 already running
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}

		queueDepth := len(p.specCh)
		currentWorkers := atomic.LoadInt32(&p.workers)
		rateLimited := atomic.LoadInt32(&p.rateLimited)

		if rateLimited == 1 {
			// Don't scale up during rate limit
			continue
		}

		// Scale up if queue has work and we're below max
		if queueDepth > int(currentWorkers) && currentWorkers < p.maxWorkers {
			p.spawnWorker(ctx, wg, nextID)
			nextID++
			log.Printf("[scaler] scaled up: %d workers, %d queued", atomic.LoadInt32(&p.workers), queueDepth)
		}
	}
}

// runHealer checks health every 60s and fixes issues.
func (p *Pipeline) runHealer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(60 * time.Second):
		}

		// Check port forwards
		if !portAlive(5432) {
			log.Println("[healer] port-forward 5432 dead, reconnecting")
			exec.Command("kubectl", "port-forward", "pod/postgres-0", "5432:5432", "-n", "factory").Start()
		}
		if !portAlive(9090) {
			log.Println("[healer] port-forward 9090 dead, reconnecting")
			exec.Command("kubectl", "port-forward", "-n", "factory", "svc/archive-serve", "9090:9090").Start()
		}

		// Check for stuck tmux sessions (> 30 min)
		out, _ := exec.Command("tmux", "list-sessions", "-F", "#{session_name} #{session_created}").CombinedOutput()
		now := time.Now().Unix()
		for _, line := range strings.Split(string(out), "\n") {
			parts := strings.Fields(line)
			if len(parts) < 2 || !strings.HasPrefix(parts[0], "build-") {
				continue
			}
			var created int64
			fmt.Sscanf(parts[1], "%d", &created)
			if created > 0 && now-created > 1800 {
				log.Printf("[healer] killing stuck session %s (age: %dm)", parts[0], (now-created)/60)
				exec.Command("tmux", "kill-session", "-t", parts[0]).Run()
			}
		}

		// Refresh Claude creds if needed
		exec.Command("claude", "-p", "ok", "--max-turns", "1", "--output-format", "text").Run()
	}
}

func isRateLimit(errStr string) bool {
	lower := strings.ToLower(errStr)
	return strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "usage limit") ||
		strings.Contains(lower, "hit your limit") ||
		strings.Contains(lower, "too many requests") ||
		strings.Contains(lower, "429")
}

func parseResetTime(errStr string) time.Time {
	// Try to find "resets Xpm" or "resets Xam" pattern
	lower := strings.ToLower(errStr)
	for _, pattern := range []string{"resets ", "reset at ", "retry after "} {
		if idx := strings.Index(lower, pattern); idx >= 0 {
			timeStr := lower[idx+len(pattern):]
			if len(timeStr) > 10 {
				timeStr = timeStr[:10]
			}
			// Try common formats
			for _, layout := range []string{"3pm", "3:04pm", "15:04"} {
				if t, err := time.Parse(layout, strings.TrimSpace(timeStr)); err == nil {
					now := time.Now()
					reset := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
					if reset.Before(now) {
						reset = reset.Add(24 * time.Hour)
					}
					return reset
				}
			}
		}
	}
	// Default: wait 30 minutes
	return time.Now().Add(30 * time.Minute)
}

func portAlive(port int) bool {
	out, _ := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port)).CombinedOutput()
	return len(strings.TrimSpace(string(out))) > 0
}
