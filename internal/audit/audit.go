package audit

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
)

// Auditor scores and manages shipped repos.
type Auditor struct {
	cfg *config.Config
	db  *db.Store
}

// New creates an Auditor.
func New(cfg *config.Config, store *db.Store) *Auditor {
	return &Auditor{cfg: cfg, db: store}
}

// RunAll audits all shipped repos.
func (a *Auditor) RunAll(ctx context.Context) error {
	names, err := a.db.ShippedRepoNames(ctx)
	if err != nil {
		return fmt.Errorf("get shipped repos: %w", err)
	}

	log.Printf("[audit] auditing %d shipped repos", len(names))

	for _, name := range names {
		score, err := a.Score(ctx, name)
		if err != nil {
			log.Printf("[audit] %s: error: %v", name, err)
			continue
		}
		log.Printf("[audit] %s: score=%d", name, score)

		if score < 50 {
			log.Printf("[audit] %s: score too low, deleting", name)
			if err := a.Delete(ctx, name); err != nil {
				log.Printf("[audit] delete %s: %v", name, err)
			}
		}
	}

	return nil
}

// Score clones a repo from GitHub and scores it 0-100.
func (a *Auditor) Score(ctx context.Context, repoName string) (int, error) {
	tmpDir := filepath.Join(os.TempDir(), "factory-audit", repoName)
	defer os.RemoveAll(tmpDir)

	// Clone
	url := fmt.Sprintf("https://github.com/%s/%s.git", a.cfg.GitHubUser, repoName)
	clone := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, tmpDir)
	if out, err := clone.CombinedOutput(); err != nil {
		return 0, fmt.Errorf("clone failed: %s", string(out))
	}

	score := 0
	as := db.AuditScore{RepoName: repoName}

	// Has README? (+15)
	if _, err := os.Stat(filepath.Join(tmpDir, "README.md")); err == nil {
		score += 15
		as.HasReadme = true
	}

	// Has tests? (+15)
	hasTests := false
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".test.ts") ||
			strings.HasSuffix(name, ".test.js") || strings.HasPrefix(name, "test_") {
			hasTests = true
		}
		return nil
	})
	if hasTests {
		score += 15
		as.HasTests = true
	}

	// Has references in README? (+10)
	if readme, err := os.ReadFile(filepath.Join(tmpDir, "README.md")); err == nil {
		content := string(readme)
		if strings.Contains(content, "arxiv") || strings.Contains(content, "arXiv") ||
			strings.Contains(content, "github.com/") {
			score += 10
			as.HasReferences = true
		}
	}

	// Correct module path? (+10)
	if gomod, err := os.ReadFile(filepath.Join(tmpDir, "go.mod")); err == nil {
		expected := fmt.Sprintf("github.com/%s/%s", a.cfg.GitHubUser, repoName)
		if strings.Contains(string(gomod), expected) {
			score += 10
			as.CorrectModulePath = true
		}
	} else {
		// Not a Go project, give credit
		score += 10
		as.CorrectModulePath = true
	}

	// Compiles? (+25)
	buildCmd := exec.CommandContext(ctx, "make", "build")
	buildCmd.Dir = tmpDir
	if buildCmd.Run() == nil {
		score += 25
		as.Compiles = true
	}

	// Tests pass? (+25)
	testCmd := exec.CommandContext(ctx, "make", "test")
	testCmd.Dir = tmpDir
	if testCmd.Run() == nil {
		score += 25
		as.TestsPass = true
	}

	as.Score = score

	// Store the audit score
	if err := a.db.UpsertAuditScore(ctx, as); err != nil {
		log.Printf("[audit] store score for %s: %v", repoName, err)
	}

	return score, nil
}

// Delete removes a repo from GitHub.
func (a *Auditor) Delete(ctx context.Context, repoName string) error {
	cmd := exec.CommandContext(ctx, "gh", "repo", "delete",
		fmt.Sprintf("%s/%s", a.cfg.GitHubUser, repoName),
		"--yes",
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GH_TOKEN=%s", a.cfg.GitHubToken))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete repo: %s: %w", string(out), err)
	}
	return nil
}
