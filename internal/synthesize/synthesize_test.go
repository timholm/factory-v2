package synthesize

import (
	"testing"
)

func TestParseSpecFromOutput(t *testing.T) {
	output := `Here is the product spec:

{
  "name": "neural-fuse",
  "description": "A tool that fuses neural techniques",
  "language": "go",
  "papers": [{"id": "2401.00001", "title": "Paper 1"}],
  "repos": [{"full_name": "owner/repo", "url": "https://github.com/owner/repo"}],
  "techniques": ["transformer", "diffusion"],
  "technique_map": {"transformer": "core attention", "diffusion": "generation"},
  "architecture": "Pipeline architecture",
  "features": ["feature 1", "feature 2"]
}

That's the spec.`

	spec, err := parseSpecFromOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.Name != "neural-fuse" {
		t.Errorf("expected name 'neural-fuse', got '%s'", spec.Name)
	}
	if spec.Language != "go" {
		t.Errorf("expected language 'go', got '%s'", spec.Language)
	}
	if len(spec.Papers) != 1 {
		t.Errorf("expected 1 paper, got %d", len(spec.Papers))
	}
	if len(spec.Techniques) != 2 {
		t.Errorf("expected 2 techniques, got %d", len(spec.Techniques))
	}
	if len(spec.TechniqueMap) != 2 {
		t.Errorf("expected 2 technique_map entries, got %d", len(spec.TechniqueMap))
	}
}

func TestParseSpecFromOutput_NoJSON(t *testing.T) {
	output := "This is just prose with no JSON at all."
	_, err := parseSpecFromOutput(output)
	if err == nil {
		t.Fatal("expected error for output without JSON")
	}
}

func TestParseSpecFromOutput_NoName(t *testing.T) {
	output := `{"description": "no name field"}`
	_, err := parseSpecFromOutput(output)
	if err == nil {
		t.Fatal("expected error for spec without name")
	}
}

func TestLoadSpec_NonexistentFile(t *testing.T) {
	_, err := LoadSpec("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestCleanEnv(t *testing.T) {
	env := cleanEnv()

	for _, e := range env {
		if len(e) > 13 && e[:13] == "GITHUB_TOKEN=" {
			t.Error("GITHUB_TOKEN should be stripped")
		}
		if len(e) > 18 && e[:18] == "ANTHROPIC_API_KEY=" {
			t.Error("ANTHROPIC_API_KEY should be stripped")
		}
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if truncate(short, 10) != "hello" {
		t.Error("short string should not be truncated")
	}

	long := "hello world this is a long string"
	result := truncate(long, 10)
	if len(result) > 13 { // 10 + "..."
		t.Errorf("expected truncated string, got: %s", result)
	}
}
