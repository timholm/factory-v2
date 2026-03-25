package research

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Repo represents a GitHub repository found during research.
type Repo struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	URL         string `json:"html_url"`
	Stars       int    `json:"stargazers_count"`
	Language    string `json:"language"`
}

type searchResult struct {
	Items []Repo `json:"items"`
}

// SearchRepos finds GitHub repos related to a problem space.
func SearchRepos(ctx context.Context, token, problemSpace string, n int) ([]Repo, error) {
	query := url.QueryEscape(problemSpace + " stars:>10")
	apiURL := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&per_page=%d", query, n)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github search returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result searchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search result: %w", err)
	}

	return result.Items, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
