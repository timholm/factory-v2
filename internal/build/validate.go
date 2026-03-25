package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/timholm/factory-v2/internal/synthesize"
)

// Validate checks that a build meets quality requirements.
func Validate(workDir string, spec *synthesize.ProductSpec, githubUser string) error {
	var errors []string

	// Check module path (Go only)
	if spec.Language == "go" || spec.Language == "" {
		expectedModule := fmt.Sprintf("github.com/%s/%s", githubUser, spec.Name)
		gomod, err := os.ReadFile(filepath.Join(workDir, "go.mod"))
		if err != nil {
			errors = append(errors, "go.mod not found")
		} else if !strings.Contains(string(gomod), expectedModule) {
			errors = append(errors, fmt.Sprintf("go.mod has wrong module path (expected %s)", expectedModule))
		}
	}

	// Check README exists
	if !fileExists(filepath.Join(workDir, "README.md")) {
		errors = append(errors, "README.md not found")
	}

	// Check README has paper references
	if readme, err := os.ReadFile(filepath.Join(workDir, "README.md")); err == nil {
		content := string(readme)
		refCount := 0
		for _, p := range spec.Papers {
			if strings.Contains(content, p.ID) || strings.Contains(content, p.Title) {
				refCount++
			}
		}
		if refCount < 3 {
			errors = append(errors, fmt.Sprintf("README references only %d papers (need at least 3)", refCount))
		}
	}

	// Check for test files
	hasTests := false
	_ = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
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
	if !hasTests {
		errors = append(errors, "no test files found")
	}

	// Check no secrets leaked
	if err := checkNoSecrets(workDir); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func checkNoSecrets(dir string) error {
	patterns := []string{"ghp_", "gho_", "sk-ant-", "GITHUB_TOKEN", "ANTHROPIC_API_KEY"}

	var found []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip binary files and git
		if strings.Contains(path, ".git/") || strings.HasSuffix(path, ".exe") ||
			strings.HasSuffix(path, ".bin") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := string(data)
		for _, p := range patterns {
			if strings.Contains(content, p) {
				// Allow references to the pattern itself (like in secret-scrubbing code)
				rel, _ := filepath.Rel(dir, path)
				if !strings.HasSuffix(rel, "secrets.go") && !strings.HasSuffix(rel, "validate.go") {
					found = append(found, fmt.Sprintf("%s contains %s", rel, p))
				}
			}
		}
		return nil
	})

	if len(found) > 0 {
		return fmt.Errorf("secrets detected:\n  %s", strings.Join(found, "\n  "))
	}
	return nil
}
