package discover

import (
	"testing"
)

func TestSelectDiverse(t *testing.T) {
	papers := []Paper{
		{ID: "1", Title: "Transformer Attention Mechanisms", Abstract: "We present a novel attention mechanism for transformers."},
		{ID: "2", Title: "Diffusion Models for Image Generation", Abstract: "Denoising diffusion probabilistic models generate images."},
		{ID: "3", Title: "Reinforcement Learning from Human Feedback", Abstract: "RLHF improves language model alignment."},
		{ID: "4", Title: "Graph Neural Networks for Molecules", Abstract: "GNN message passing predicts molecular properties."},
		{ID: "5", Title: "Contrastive Learning for Vision", Abstract: "SimCLR uses contrastive learning for visual representations."},
		{ID: "6", Title: "Generative Adversarial Networks", Abstract: "GANs generate realistic synthetic data."},
		{ID: "7", Title: "Federated Learning Privacy", Abstract: "Privacy-preserving distributed machine learning."},
		{ID: "8", Title: "Meta-Learning Few Shot", Abstract: "MAML enables few-shot learning via meta-learning."},
		{ID: "9", Title: "Neural Architecture Search", Abstract: "Automated search for optimal neural network architectures."},
		{ID: "10", Title: "Transformer Attention Improvements", Abstract: "Improved multi-head attention for transformers with efficiency."},
	}

	selected := selectDiverse(papers, 7)

	if len(selected) != 7 {
		t.Fatalf("expected 7 papers, got %d", len(selected))
	}

	// Check no duplicates
	seen := make(map[string]bool)
	for _, p := range selected {
		if seen[p.ID] {
			t.Errorf("duplicate paper ID: %s", p.ID)
		}
		seen[p.ID] = true
	}
}

func TestSelectDiverseSmallInput(t *testing.T) {
	papers := []Paper{
		{ID: "1", Title: "Paper A", Abstract: "Abstract A"},
		{ID: "2", Title: "Paper B", Abstract: "Abstract B"},
		{ID: "3", Title: "Paper C", Abstract: "Abstract C"},
	}

	selected := selectDiverse(papers, 7)

	if len(selected) != 3 {
		t.Fatalf("expected 3 papers (all of them), got %d", len(selected))
	}
}

func TestTfVector(t *testing.T) {
	vec := tfVector("hello world hello")

	if _, ok := vec["hello"]; !ok {
		t.Error("expected 'hello' in vector")
	}
	if _, ok := vec["world"]; !ok {
		t.Error("expected 'world' in vector")
	}

	// 'hello' appears twice, should have higher raw count (but normalized)
	// Just check both exist
}

func TestCosineDist(t *testing.T) {
	a := map[string]float64{"hello": 1.0}
	b := map[string]float64{"hello": 1.0}

	dist := cosineDist(a, b)
	if dist > 0.001 {
		t.Errorf("expected ~0 distance for identical vectors, got %f", dist)
	}

	c := map[string]float64{"world": 1.0}
	dist2 := cosineDist(a, c)
	if dist2 < 0.999 {
		t.Errorf("expected ~1 distance for orthogonal vectors, got %f", dist2)
	}
}

func TestGroupByCategory(t *testing.T) {
	papers := []Paper{
		{ID: "1", Category: "cs.AI"},
		{ID: "2", Category: "cs.AI"},
		{ID: "3", Category: "cs.LG"},
		{ID: "4", Category: ""},
	}

	groups := groupByCategory(papers)

	if len(groups["cs.AI"]) != 2 {
		t.Errorf("expected 2 cs.AI papers, got %d", len(groups["cs.AI"]))
	}
	if len(groups["cs.LG"]) != 1 {
		t.Errorf("expected 1 cs.LG paper, got %d", len(groups["cs.LG"]))
	}
	if len(groups["general"]) != 1 {
		t.Errorf("expected 1 general paper, got %d", len(groups["general"]))
	}
}
