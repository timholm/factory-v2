package research

import (
	"testing"

	"github.com/timholm/factory-v2/internal/discover"
)

func TestExtractTechnique_Transformer(t *testing.T) {
	paper := discover.Paper{
		ID:       "2401.00001",
		Title:    "Efficient Multi-Head Attention for Transformers",
		Abstract: "We propose a novel self-attention mechanism that reduces the quadratic complexity of transformer models. Our multi-head attention approach achieves state-of-the-art results.",
	}

	tech := ExtractTechnique(paper)

	if tech.Name != "transformer" {
		t.Errorf("expected technique 'transformer', got '%s'", tech.Name)
	}
	if tech.PaperID != "2401.00001" {
		t.Errorf("expected paper ID '2401.00001', got '%s'", tech.PaperID)
	}
	if len(tech.Keywords) == 0 {
		t.Error("expected at least one keyword")
	}
}

func TestExtractTechnique_Diffusion(t *testing.T) {
	paper := discover.Paper{
		ID:       "2401.00002",
		Title:    "Denoising Diffusion Probabilistic Models for 3D",
		Abstract: "We extend DDPM to 3D generation using denoising diffusion with score-based sampling.",
	}

	tech := ExtractTechnique(paper)

	if tech.Name != "diffusion" {
		t.Errorf("expected technique 'diffusion', got '%s'", tech.Name)
	}
}

func TestExtractTechnique_Unknown(t *testing.T) {
	paper := discover.Paper{
		ID:       "2401.00003",
		Title:    "Novel Approach to Widget Manufacturing",
		Abstract: "We present a completely new method for manufacturing widgets at scale.",
	}

	tech := ExtractTechnique(paper)

	if tech.Name != "novel-method" {
		t.Errorf("expected technique 'novel-method', got '%s'", tech.Name)
	}
}

func TestSummarize(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence. Fourth sentence."
	result := summarize(text, 2)

	if result != "First sentence. Second sentence." {
		t.Errorf("unexpected summary: %s", result)
	}
}

func TestSummarizeShort(t *testing.T) {
	text := "Only one sentence."
	result := summarize(text, 5)

	if result != "Only one sentence." {
		t.Errorf("unexpected summary: %s", result)
	}
}

func TestExtractTopWords(t *testing.T) {
	text := "machine learning machine learning deep learning neural network neural"
	words := extractTopWords(text, 3)

	if len(words) == 0 {
		t.Fatal("expected at least one word")
	}

	// "machine" and "learning" and "neural" should be top words
	found := make(map[string]bool)
	for _, w := range words {
		found[w] = true
	}
	if !found["machine"] && !found["learning"] && !found["neural"] {
		t.Errorf("expected common words in top words, got %v", words)
	}
}
