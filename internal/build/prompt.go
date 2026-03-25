package build

import (
	"fmt"
	"strings"

	"github.com/timholm/factory-v2/internal/synthesize"
)

// RenderBuildPrompt creates the ONE prompt for the entire build session.
func RenderBuildPrompt(spec *synthesize.ProductSpec, githubUser string) (string, error) {
	lang := spec.Language
	if lang == "" {
		lang = "go"
	}

	var sb strings.Builder

	sb.WriteString("You are building a production-grade tool from research. Read SPEC.md for requirements.\n\n")
	sb.WriteString("SPEC.md contains:\n")
	sb.WriteString("- 7 research papers with their techniques — implement ALL of them\n")
	sb.WriteString("- 7 GitHub repos — learn from them, improve on them\n\n")

	sb.WriteString("Do ALL of the following in this session:\n\n")
	sb.WriteString("1. Read SPEC.md carefully\n")
	sb.WriteString("2. Implement the core code (combine techniques from all 7 papers)\n")
	sb.WriteString("3. Write real tests (at least 3 test files with assertions that pass)\n")
	sb.WriteString("4. Run `make test` — if tests fail, fix them\n")
	sb.WriteString("5. Write README.md with: description, install, usage, references to all 7 papers + 7 repos\n")
	sb.WriteString("6. Write CLAUDE.md with build/test commands and architecture\n")
	sb.WriteString("7. Write AGENTS.md with key files and how to extend\n")
	sb.WriteString("8. Run `make test` one final time — MUST pass\n\n")

	sb.WriteString(fmt.Sprintf("Module path: github.com/%s/%s\n", githubUser, spec.Name))
	sb.WriteString(fmt.Sprintf("Language: %s\n\n", lang))

	sb.WriteString("Do not stop until make test passes and all documentation exists.\n")

	return sb.String(), nil
}
