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

	// ONE Claude session to build everything
	log.Printf("[build] invoking Claude for %s (max 30 turns)", spec.Name)
	if err := invokeClaude(ctx, b.cfg.ClaudeBinary, prompt, workDir, 30, false); err != nil {
		return fmt.Errorf("claude build: %w", err)
	}

	// Resolve deps
	log.Printf("[build] resolving deps for %s", spec.Name)
	if err := ResolveDeps(workDir, spec.Language); err != nil {
		log.Printf("[build] dep resolution warning: %v", err)
	}

	// Check if tests pass
	if !TestsPass(workDir) {
		// Get test errors and retry
		errors := GetTestErrors(workDir)
		retryPrompt := fmt.Sprintf("Tests failed with the following errors. Fix them:\n\n%s", errors)
		log.Printf("[build] tests failed for %s, retrying with 15 turns", spec.Name)
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

// invokeClaude calls the Claude CLI.
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

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = cleanEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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
