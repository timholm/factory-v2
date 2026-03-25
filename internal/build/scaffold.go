package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/timholm/factory-v2/internal/synthesize"
)

// Scaffold creates the initial workspace for a build.
func Scaffold(workDir string, spec *synthesize.ProductSpec, githubUser string) error {
	modulePath := fmt.Sprintf("github.com/%s/%s", githubUser, spec.Name)

	// SPEC.md — the full spec for Claude to read
	specMD := renderSpecMD(spec)
	if err := writeFile(workDir, "SPEC.md", specMD); err != nil {
		return err
	}

	// CLAUDE.md — build instructions
	claudeMD := fmt.Sprintf(`# %s

## Build & Test
- `+"`make build`"+` — compile
- `+"`make test`"+` — run all tests
- `+"`go test ./... -v`"+` — verbose tests

## Architecture
%s

## Module
%s
`, spec.Name, spec.Architecture, modulePath)
	if err := writeFile(workDir, "CLAUDE.md", claudeMD); err != nil {
		return err
	}

	// go.mod (for Go projects)
	if spec.Language == "go" || spec.Language == "" {
		gomod := fmt.Sprintf("module %s\n\ngo 1.22\n", modulePath)
		if err := writeFile(workDir, "go.mod", gomod); err != nil {
			return err
		}
	}

	// Makefile
	makefile := renderMakefile(spec.Language)
	if err := writeFile(workDir, "Makefile", makefile); err != nil {
		return err
	}

	// spec.json (machine-readable)
	specJSON, _ := json.MarshalIndent(spec, "", "  ")
	if err := writeFile(workDir, "spec.json", string(specJSON)); err != nil {
		return err
	}

	// .gitignore
	gitignore := "*.exe\n*.dll\n*.so\n*.dylib\n*.test\n*.out\nvendor/\nnode_modules/\n.env\n*.log\ntmp/\n"
	if err := writeFile(workDir, ".gitignore", gitignore); err != nil {
		return err
	}

	return nil
}

func renderSpecMD(spec *synthesize.ProductSpec) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", spec.Name))
	sb.WriteString(fmt.Sprintf("%s\n\n", spec.Description))
	sb.WriteString(fmt.Sprintf("**Language:** %s\n\n", spec.Language))

	sb.WriteString("## Architecture\n\n")
	sb.WriteString(spec.Architecture + "\n\n")

	sb.WriteString("## Features\n\n")
	for _, f := range spec.Features {
		sb.WriteString(fmt.Sprintf("- %s\n", f))
	}
	sb.WriteString("\n")

	sb.WriteString("## Research Papers (implement techniques from ALL of these)\n\n")
	for i, p := range spec.Papers {
		sb.WriteString(fmt.Sprintf("%d. **%s** (arXiv: %s)\n", i+1, p.Title, p.ID))
	}
	sb.WriteString("\n")

	sb.WriteString("## Technique Map (how each technique is used)\n\n")
	for tech, usage := range spec.TechniqueMap {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tech, usage))
	}
	sb.WriteString("\n")

	sb.WriteString("## Reference Repos (learn from these, improve on them)\n\n")
	for i, r := range spec.Repos {
		sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, r.FullName, r.URL))
	}
	sb.WriteString("\n")

	return sb.String()
}

func renderMakefile(language string) string {
	switch language {
	case "python":
		return `.PHONY: build test clean

build:
	@echo "Python project — no build step"

test:
	python -m pytest -v

clean:
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
`
	case "typescript", "ts":
		return `.PHONY: build test clean

build:
	npx tsc

test:
	npx jest --passWithNoTests

clean:
	rm -rf dist node_modules
`
	default: // go
		return `.PHONY: build test clean

build:
	go build ./...

test:
	go test ./... -v

clean:
	rm -f bin/*
`
	}
}

func writeFile(dir, name, content string) error {
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
