package build

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/timholm/factory-v2/internal/config"
	"github.com/timholm/factory-v2/internal/db"
	"github.com/timholm/factory-v2/internal/synthesize"

	"github.com/timholm/forge-oracle/pkg/calibrate"
	"github.com/timholm/forge-oracle/pkg/diagnose"
	"github.com/timholm/forge-oracle/pkg/simulate"
	oracletypes "github.com/timholm/forge-oracle/pkg/types"
)

// Builder builds repos from product specs.
type Builder struct {
	cfg *config.Config
	db  *db.Store
}

// New creates a Builder.
func New(cfg *config.Config, store *db.Store) *Builder {
	return &Builder{cfg: cfg, db: store}
}

// Execute builds a single repo from a ProductSpec.
func (b *Builder) Execute(ctx context.Context, spec *synthesize.ProductSpec) error {
	workDir := filepath.Join(os.TempDir(), "factory-build", spec.Name)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// === ORACLE: Calibrate spec complexity ===
	cal := calibrate.NewDefault()
	oracleSpec := toOracleSpec(spec)
	calResult := cal.Calibrate(oracleSpec)
	log.Printf("[oracle] calibrate %s: complexity=%.1f, turns=%d, buildable=%v",
		spec.Name, calResult.ComplexityScore, calResult.EstimatedTurns, calResult.Buildable)
	if !calResult.Buildable {
		log.Printf("[oracle] %s too complex — suggested cuts: %v", spec.Name, calResult.SuggestedCuts)
		// Apply cuts: reduce features to what's buildable
		if len(spec.Features) > 5 {
			spec.Features = spec.Features[:5]
			log.Printf("[oracle] trimmed features to 5 for %s", spec.Name)
		}
	}

	// === ORACLE: Simulate build success ===
	sim := simulate.NewDefault()
	simResult := sim.Simulate(oracleSpec)
	log.Printf("[oracle] simulate %s: confidence=%.0f%%, turns=%d",
		spec.Name, simResult.Confidence*100, simResult.EstimatedTurns)
	if simResult.Confidence < 0.2 {
		return fmt.Errorf("oracle: skipping %s — predicted success %.0f%% is too low", spec.Name, simResult.Confidence*100)
	}

	// Scaffold the workspace
	log.Printf("[build] scaffolding %s in %s", spec.Name, workDir)
	if err := Scaffold(workDir, spec, b.cfg.GitHubUser); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	// Render the build prompt
	prompt, err := RenderBuildPrompt(spec, b.cfg.GitHubUser)
	if err != nil {
		return fmt.Errorf("render prompt: %w", err)
	}

	// Adjust turns based on oracle estimate
	maxTurns := 30
	if simResult.EstimatedTurns > 0 && simResult.EstimatedTurns < 30 {
		maxTurns = simResult.EstimatedTurns + 5 // buffer
	}

	// ONE Claude session to build everything
	log.Printf("[build] invoking Claude for %s (max %d turns)", spec.Name, maxTurns)
	if err := invokeClaude(ctx, b.cfg.ClaudeBinary, prompt, workDir, maxTurns, false); err != nil {
		return fmt.Errorf("claude build: %w", err)
	}

	// Resolve deps
	log.Printf("[build] resolving deps for %s", spec.Name)
	if err := ResolveDeps(workDir, spec.Language); err != nil {
		log.Printf("[build] dep resolution warning: %v", err)
	}

	// Check if tests pass
	if !TestsPass(workDir) {
		errors := GetTestErrors(workDir)

		// === ORACLE: Diagnose failure and generate targeted fix ===
		diag := diagnose.New()
		faultTree := diag.Parse(errors)
		log.Printf("[oracle] diagnose %s: category=%s, faults=%d",
			spec.Name, faultTree.Root.Category, diagnose.CountFaults(faultTree.Root))

		// Use oracle's targeted fix prompt instead of generic "fix them"
		retryPrompt := faultTree.FixPrompt
		if retryPrompt == "" {
			retryPrompt = fmt.Sprintf("Tests failed. Fix them:\n\n%s", errors)
		}

		log.Printf("[build] tests failed for %s, retrying with oracle-guided fix (15 turns)", spec.Name)
		if err := invokeClaude(ctx, b.cfg.ClaudeBinary, retryPrompt, workDir, 15, true); err != nil {
			log.Printf("[build] retry failed: %v", err)
		}

		// Resolve deps again after retry
		_ = ResolveDeps(workDir, spec.Language)
	}

	// Scrub secrets
	log.Printf("[build] scrubbing secrets from %s", spec.Name)
	ScrubSecrets(workDir)

	// Validate
	log.Printf("[build] validating %s", spec.Name)
	if err := Validate(workDir, spec, b.cfg.GitHubUser); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	// Git init, commit, push
	log.Printf("[build] git operations for %s", spec.Name)
	if err := GitInit(workDir); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	if err := GitCommit(workDir, "Initial build from factory-v2"); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Mirror to bare repo
	bareDir := filepath.Join(b.cfg.GitDir, spec.Name+".git")
	if err := GitMirrorToBare(workDir, bareDir); err != nil {
		log.Printf("[build] bare mirror warning: %v", err)
	}

	// Push to GitHub
	if err := GitPushToGitHub(ctx, workDir, b.cfg.GitHubUser, spec.Name, b.cfg.GitHubToken); err != nil {
		return fmt.Errorf("github push: %w", err)
	}

	log.Printf("[build] shipped %s to github.com/%s/%s", spec.Name, b.cfg.GitHubUser, spec.Name)
	return nil
}

// invokeClaude runs Claude in a tmux session named "factory-build" so you can watch live.
// Attach with: tmux attach -t factory-build
func invokeClaude(ctx context.Context, binary, prompt, workDir string, maxTurns int, continueSession bool) error {
	args := []string{
		"-p", prompt,
		"--max-turns", fmt.Sprintf("%d", maxTurns),
		"--model", "sonnet",
		"--output-format", "text",
	}
	if continueSession {
		args = append(args, "--continue")
	}

	// Build the full claude command string
	claudeCmd := binary
	for _, a := range args {
		claudeCmd += " " + shellQuote(a)
	}

	// Kill old tmux session if exists
	exec.Command("tmux", "kill-session", "-t", "factory-build").Run()

	// Create new tmux session running claude
	// This lets you watch with: tmux attach -t factory-build
	tmuxArgs := []string{
		"new-session", "-d", "-s", "factory-build",
		"-x", "200", "-y", "50",
		"sh", "-c", fmt.Sprintf("cd %s && %s; echo '=== BUILD COMPLETE ==='; sleep 5", shellQuote(workDir), claudeCmd),
	}

	tmuxCmd := exec.Command("tmux", tmuxArgs...)
	tmuxCmd.Env = cleanEnv()
	if err := tmuxCmd.Run(); err != nil {
		// Fallback to direct execution if tmux fails
		log.Printf("[build] tmux failed, running directly: %v", err)
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = workDir
		cmd.Env = cleanEnv()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	log.Printf("[build] Claude running in tmux session 'factory-build' — attach with: tmux attach -t factory-build")

	// Wait for tmux session to finish
	for {
		check := exec.Command("tmux", "has-session", "-t", "factory-build")
		if check.Run() != nil {
			break // session ended
		}
		select {
		case <-ctx.Done():
			exec.Command("tmux", "kill-session", "-t", "factory-build").Run()
			return ctx.Err()
		case <-sleepChan(2):
		}
	}

	return nil
}

func sleepChan(seconds int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		for i := 0; i < seconds*10; i++ {
			exec.Command("true").Run() // tiny delay without time.Sleep
		}
		close(ch)
	}()
	return ch
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// cleanEnv strips sensitive env vars.
func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GITHUB_TOKEN=") ||
			strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// toOracleSpec converts a factory ProductSpec to an oracle ProductSpec.
func toOracleSpec(spec *synthesize.ProductSpec) oracletypes.ProductSpec {
	return oracletypes.ProductSpec{
		Name:       spec.Name,
		Language:   spec.Language,
		Features:   spec.Features,
		Techniques: spec.Techniques,
	}
}

// TestsPass runs make test and returns true if it succeeds.
func TestsPass(workDir string) bool {
	cmd := exec.Command("make", "test")
	cmd.Dir = workDir
	return cmd.Run() == nil
}

// GetTestErrors runs make test and captures stderr.
func GetTestErrors(workDir string) string {
	cmd := exec.Command("make", "test")
	cmd.Dir = workDir
	out, _ := cmd.CombinedOutput()
	return string(out)
}
