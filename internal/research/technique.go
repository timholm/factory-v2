package research

import (
	"strings"

	"github.com/timholm/factory-v2/internal/discover"
)

// Technique represents a key technique extracted from a paper.
type Technique struct {
	PaperID     string
	PaperTitle  string
	Name        string
	Description string
	Keywords    []string
}

// techniqueKeywords maps technique categories to their indicators.
var techniqueKeywords = map[string][]string{
	"transformer":        {"transformer", "attention", "self-attention", "multi-head"},
	"diffusion":          {"diffusion", "denoising", "score-based", "ddpm"},
	"reinforcement":      {"reinforcement", "reward", "policy", "q-learning", "rlhf"},
	"graph-neural":       {"graph neural", "gnn", "message passing", "node embedding"},
	"contrastive":        {"contrastive", "siamese", "triplet", "simclr", "clip"},
	"generative":         {"generative", "gan", "vae", "variational"},
	"federated":          {"federated", "distributed learning", "privacy-preserving"},
	"meta-learning":      {"meta-learning", "few-shot", "maml", "prototypical"},
	"neuro-symbolic":     {"neuro-symbolic", "symbolic reasoning", "logic neural"},
	"optimization":       {"optimization", "gradient", "adam", "sgd", "convergence"},
	"retrieval":          {"retrieval", "rag", "dense retrieval", "embedding search"},
	"pruning":            {"pruning", "quantization", "distillation", "compression"},
	"multimodal":         {"multimodal", "vision-language", "cross-modal"},
	"reasoning":          {"reasoning", "chain-of-thought", "step-by-step", "planning"},
	"code-generation":    {"code generation", "program synthesis", "code completion"},
	"anomaly-detection":  {"anomaly", "outlier", "out-of-distribution"},
	"time-series":        {"time series", "forecasting", "temporal"},
	"embedding":          {"embedding", "representation learning", "vector space"},
}

// ExtractTechnique extracts the key technique from a paper using keyword matching.
func ExtractTechnique(paper discover.Paper) Technique {
	text := strings.ToLower(paper.Title + " " + paper.Abstract)

	bestTechnique := ""
	bestScore := 0
	var bestKeywords []string

	for tech, keywords := range techniqueKeywords {
		score := 0
		var matched []string
		for _, kw := range keywords {
			if strings.Contains(text, kw) {
				score++
				matched = append(matched, kw)
			}
		}
		if score > bestScore {
			bestScore = score
			bestTechnique = tech
			bestKeywords = matched
		}
	}

	if bestTechnique == "" {
		bestTechnique = "novel-method"
		bestKeywords = extractTopWords(text, 5)
	}

	// Build description from first 2 sentences of abstract
	desc := summarize(paper.Abstract, 2)

	return Technique{
		PaperID:     paper.ID,
		PaperTitle:  paper.Title,
		Name:        bestTechnique,
		Description: desc,
		Keywords:    bestKeywords,
	}
}

// summarize returns the first n sentences of text.
func summarize(text string, n int) string {
	sentences := strings.SplitAfter(text, ".")
	if len(sentences) > n {
		sentences = sentences[:n]
	}
	return strings.TrimSpace(strings.Join(sentences, ""))
}

// extractTopWords returns the n most frequent non-stopword tokens.
func extractTopWords(text string, n int) []string {
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "is": true, "are": true, "was": true,
		"were": true, "be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "can": true, "this": true,
		"that": true, "these": true, "those": true, "it": true, "its": true, "we": true,
		"our": true, "they": true, "their": true, "not": true, "which": true, "what": true,
	}

	freq := make(map[string]int)
	for _, w := range strings.Fields(text) {
		w = strings.Trim(w, ".,;:!?()[]{}\"'")
		if len(w) < 3 || stopwords[w] {
			continue
		}
		freq[w]++
	}

	type wordCount struct {
		word  string
		count int
	}
	var wcs []wordCount
	for w, c := range freq {
		wcs = append(wcs, wordCount{w, c})
	}

	// Simple sort by count descending
	for i := 0; i < len(wcs); i++ {
		for j := i + 1; j < len(wcs); j++ {
			if wcs[j].count > wcs[i].count {
				wcs[i], wcs[j] = wcs[j], wcs[i]
			}
		}
	}

	var result []string
	for i := 0; i < n && i < len(wcs); i++ {
		result = append(result, wcs[i].word)
	}
	return result
}
