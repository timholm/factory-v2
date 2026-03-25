package synthesize

import (
	"fmt"
	"strings"

	"github.com/timholm/factory-v2/internal/research"
)

// buildSynthesisPrompt creates the prompt for Claude Opus to fuse research into a product spec.
func buildSynthesisPrompt(res *research.ResearchResult) string {
	var sb strings.Builder

	sb.WriteString("You are a product architect for an autonomous software factory.\n\n")
	sb.WriteString("The factory is a pipeline that: reads arXiv papers → generates product specs → builds Go/Python repos → ships to GitHub.\n")
	sb.WriteString("It uses: Claude Code CLI, PostgreSQL + pgvector, a K8s cluster, llm-router for cost optimization.\n\n")
	sb.WriteString("Your job: fuse 7 research techniques into ONE tool that DIRECTLY IMPROVES this factory pipeline.\n")
	sb.WriteString("The tool must help the factory: build better code, write better tests, find better papers,\n")
	sb.WriteString("generate better specs, optimize prompts, improve build throughput, or raise product quality.\n\n")
	sb.WriteString("If you cannot find a way to make the tool useful to the factory, find the closest connection.\n")
	sb.WriteString("For example: a reasoning technique can improve how Claude generates code.\n")
	sb.WriteString("A retrieval technique can improve how we find relevant papers.\n")
	sb.WriteString("An optimization technique can improve inference cost.\n\n")
	sb.WriteString("Problem space: " + res.ProblemSpace + "\n\n")

	sb.WriteString("## Research Papers & Techniques\n\n")
	for i, t := range res.Techniques {
		sb.WriteString(fmt.Sprintf("%d. **%s** (ID: %s)\n", i+1, t.PaperTitle, t.PaperID))
		sb.WriteString(fmt.Sprintf("   Technique: %s\n", t.Name))
		sb.WriteString(fmt.Sprintf("   Description: %s\n", t.Description))
		sb.WriteString(fmt.Sprintf("   Keywords: %s\n\n", strings.Join(t.Keywords, ", ")))
	}

	sb.WriteString("## Existing GitHub Repos\n\n")
	for i, r := range res.Repos {
		sb.WriteString(fmt.Sprintf("%d. **%s** (%s) - %d stars\n", i+1, r.FullName, r.URL, r.Stars))
		sb.WriteString(fmt.Sprintf("   %s (Language: %s)\n\n", r.Description, r.Language))
	}

	sb.WriteString("## Requirements\n\n")
	sb.WriteString("Respond with a JSON object (and nothing else) with this exact schema:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"name\": \"kebab-case-tool-name\",\n")
	sb.WriteString("  \"description\": \"One paragraph describing the tool\",\n")
	sb.WriteString("  \"language\": \"go\",\n")
	sb.WriteString("  \"papers\": [{\"id\": \"arxiv-id\", \"title\": \"paper title\"}],\n")
	sb.WriteString("  \"repos\": [{\"full_name\": \"owner/repo\", \"url\": \"https://...\"}],\n")
	sb.WriteString("  \"techniques\": [\"technique-name-1\", ...],\n")
	sb.WriteString("  \"technique_map\": {\"technique-name\": \"how it's used in the tool\"},\n")
	sb.WriteString("  \"architecture\": \"Description of the architecture\",\n")
	sb.WriteString("  \"features\": [\"feature 1\", \"feature 2\", ...],\n")
	sb.WriteString("  \"factory_integration\": \"Which part of the factory pipeline this improves and HOW\",\n")
	sb.WriteString("  \"deployment_target\": \"Which factory component this replaces or augments (e.g. idea-engine, build phase, llm-router)\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- The tool MUST use techniques from ALL 7 papers\n")
	sb.WriteString("- The technique_map MUST map each technique to how it's used\n")
	sb.WriteString("- The tool MUST improve the factory pipeline — factory_integration cannot be empty\n")
	sb.WriteString("- Language should be 'go' unless the domain strongly favors Python or TypeScript\n")
	sb.WriteString("- Name must be kebab-case, memorable, and descriptive\n")
	sb.WriteString("- Architecture must be clear enough for an AI to implement\n")
	sb.WriteString("- Features must be concrete and testable\n")

	return sb.String()
}
