package research

import (
	"context"
	"fmt"
	"log"

	"github.com/timholm/factory-v2/internal/discover"
)

// ResearchResult holds the output of investigating a cluster.
type ResearchResult struct {
	ProblemSpace string
	Papers       []discover.Paper
	Techniques   []Technique
	Repos        []Repo
}

// Researcher investigates clusters to extract techniques and find repos.
type Researcher struct {
	githubToken string
}

// New creates a Researcher.
func New(githubToken string) *Researcher {
	return &Researcher{githubToken: githubToken}
}

// Investigate takes a cluster and extracts techniques from each paper,
// then finds relevant GitHub repos for the problem space.
func (r *Researcher) Investigate(ctx context.Context, cluster discover.Cluster) (*ResearchResult, error) {
	result := &ResearchResult{
		ProblemSpace: cluster.ProblemSpace,
		Papers:       cluster.Papers,
	}

	// Extract technique from each paper
	for _, paper := range cluster.Papers {
		t := ExtractTechnique(paper)
		result.Techniques = append(result.Techniques, t)
		log.Printf("[research] paper %s: technique=%s", paper.ID, t.Name)
	}

	// Find GitHub repos related to the problem space
	repos, err := SearchRepos(ctx, r.githubToken, cluster.ProblemSpace, 7)
	if err != nil {
		return nil, fmt.Errorf("search repos: %w", err)
	}
	result.Repos = repos
	log.Printf("[research] found %d repos for %s", len(repos), cluster.ProblemSpace)

	return result, nil
}
