package synthesize

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/timholm/factory-v2/internal/research"
)

// ProductSpec is the output of synthesis: a complete spec for building a product.
type ProductSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Language    string   `json:"language"`
	Papers      []PaperRef `json:"papers"`
	Repos       []RepoRef  `json:"repos"`
	Techniques  []string `json:"techniques"`
	TechniqueMap map[string]string `json:"technique_map"`
	Architecture string  `json:"architecture"`
	Features    []string `json:"features"`
}

// PaperRef references a paper used in the spec.
type PaperRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// RepoRef references a GitHub repo used in the spec.
type RepoRef struct {
	FullName string `json:"full_name"`
	URL      string `json:"url"`
}

// Synthesizer uses Claude Opus to fuse research into a product spec.
type Synthesizer struct {
	claudeBinary string
}

// New creates a Synthesizer.
func New(claudeBinary string) *Synthesizer {
	return &Synthesizer{claudeBinary: claudeBinary}
}

// LoadSpec reads a ProductSpec from a JSON file.
func LoadSpec(path string) (*ProductSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec ProductSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// Fuse takes research results and produces a ProductSpec by calling Claude Opus.
func (s *Synthesizer) Fuse(ctx context.Context, res *research.ResearchResult) (*ProductSpec, error) {
	prompt := buildSynthesisPrompt(res)

	log.Printf("[synthesize] calling Claude Opus for %s", res.ProblemSpace)

	cmd := exec.CommandContext(ctx, s.claudeBinary,
		"-p", prompt,
		"--max-turns", "5",
		"--model", "opus",
		"--output-format", "text",
	)

	// Strip sensitive env vars
	cmd.Env = cleanEnv()

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("claude opus failed: %s\nstderr: %s", err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("claude opus: %w", err)
	}

	// Parse the JSON from Claude's output
	spec, err := parseSpecFromOutput(string(out))
	if err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	return spec, nil
}

// parseSpecFromOutput extracts JSON from Claude's output (which may contain prose around it).
func parseSpecFromOutput(output string) (*ProductSpec, error) {
	// Try to find JSON block
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")

	if start < 0 || end < 0 || end <= start {
		return nil, fmt.Errorf("no JSON found in output:\n%s", truncate(output, 500))
	}

	jsonStr := output[start : end+1]
	var spec ProductSpec
	if err := json.Unmarshal([]byte(jsonStr), &spec); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\n%s", err, truncate(jsonStr, 500))
	}

	if spec.Name == "" {
		return nil, fmt.Errorf("spec has no name")
	}

	return &spec, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// cleanEnv strips sensitive env vars before invoking Claude.
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
