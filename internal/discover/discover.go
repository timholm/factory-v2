package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Paper represents a paper from the archive API.
type Paper struct {
	ID       string `json:"arxiv_id"`
	Title    string `json:"title"`
	Abstract string `json:"abstract"`
	Category string `json:"categories"`
}

// Cluster is a group of 7 diverse papers about one problem space.
type Cluster struct {
	ProblemSpace string
	Papers       []Paper
	PaperIDs     []string
}

// Discoverer fetches papers from the archive API and clusters them.
type Discoverer struct {
	archiveURL string
	client     *http.Client
}

// New creates a Discoverer.
func New(archiveURL string) *Discoverer {
	return &Discoverer{
		archiveURL: archiveURL,
		client:     &http.Client{},
	}
}

// FindClusters fetches recent papers and clusters them by problem space.
func (d *Discoverer) FindClusters(ctx context.Context) ([]Cluster, error) {
	papers, err := d.fetchPapers(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch papers: %w", err)
	}

	// Filter to only papers relevant to factory improvement
	papers = filterFactoryRelevant(papers)

	if len(papers) == 0 {
		return nil, fmt.Errorf("no factory-relevant papers found")
	}

	log.Printf("[discover] fetched %d factory-relevant papers", len(papers))

	// Group by category
	groups := groupByCategory(papers)

	var clusters []Cluster
	for space, group := range groups {
		if len(group) < 7 {
			continue
		}

		// Pick 7 diverse papers from this group
		selected := selectDiverse(group, 7)
		var ids []string
		for _, p := range selected {
			ids = append(ids, p.ID)
		}

		clusters = append(clusters, Cluster{
			ProblemSpace: space,
			Papers:       selected,
			PaperIDs:     ids,
		})
	}

	return clusters, nil
}

func (d *Discoverer) fetchPapers(ctx context.Context) ([]Paper, error) {
	// Only categories that produce factory-improving tools
	categories := []string{"cs.SE", "cs.CL", "cs.AI", "cs.LG"} // software engineering, NLP, AI, machine learning — NO cs.CV
	var allPapers []Paper
	seen := make(map[string]bool)

	for _, cat := range categories {
		url := fmt.Sprintf("%s/papers/recent?cat=%s&days=14&limit=50", d.archiveURL, cat)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		resp, err := d.client.Do(req)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}

		// Archive API wraps in {"papers": [...]}
		var wrapped struct {
			Papers []Paper `json:"papers"`
		}
		if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped.Papers) > 0 {
			for _, p := range wrapped.Papers {
				if !seen[p.ID] {
					seen[p.ID] = true
					allPapers = append(allPapers, p)
				}
			}
			continue
		}

		// Try bare array
		var papers []Paper
		if err := json.Unmarshal(body, &papers); err == nil {
			for _, p := range papers {
				if !seen[p.ID] {
					seen[p.ID] = true
					allPapers = append(allPapers, p)
				}
			}
		}
	}

	if len(allPapers) == 0 {
		return nil, fmt.Errorf("no papers found from archive")
	}

	return allPapers, nil
}

// filterFactoryRelevant keeps only papers whose titles/abstracts relate to
// topics that can improve the factory: code generation, testing, LLM optimization,
// retrieval, agents, prompts, evaluation, embeddings, search.
func filterFactoryRelevant(papers []Paper) []Paper {
	keywords := []string{
		"code generation", "code repair", "code review", "code search",
		"test generation", "test", "testing", "software engineering",
		"prompt", "instruction tuning", "alignment",
		"retrieval", "rag", "search", "embedding", "vector",
		"agent", "tool use", "function calling", "orchestration",
		"evaluation", "benchmark", "quality", "hallucination",
		"inference", "optimization", "efficient", "speculative decoding",
		"fine-tuning", "lora", "training", "distillation",
		"reasoning", "chain-of-thought", "planning",
		"guardrail", "safety", "injection", "security",
	}

	var filtered []Paper
	for _, p := range papers {
		lower := strings.ToLower(p.Title + " " + p.Abstract)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				filtered = append(filtered, p)
				break
			}
		}
	}
	return filtered
}

func groupByCategory(papers []Paper) map[string][]Paper {
	groups := make(map[string][]Paper)
	for _, p := range papers {
		cat := p.Category
		if cat == "" {
			cat = "general"
		}
		groups[cat] = append(groups[cat], p)
	}
	return groups
}
