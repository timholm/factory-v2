package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Paper represents a paper from the archive API.
type Paper struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Abstract string `json:"abstract"`
	Category string `json:"category"`
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

	if len(papers) == 0 {
		return nil, fmt.Errorf("no papers found")
	}

	log.Printf("[discover] fetched %d papers", len(papers))

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
	url := fmt.Sprintf("%s/api/papers?limit=200", d.archiveURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("archive API returned %d: %s", resp.StatusCode, string(body))
	}

	var papers []Paper
	if err := json.NewDecoder(resp.Body).Decode(&papers); err != nil {
		return nil, fmt.Errorf("decode papers: %w", err)
	}

	return papers, nil
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
