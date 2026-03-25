package discover

import (
	"math"
	"strings"
)

// selectDiverse uses a greedy max-diversity algorithm to pick n papers
// that are as different from each other as possible (by abstract text).
func selectDiverse(papers []Paper, n int) []Paper {
	if len(papers) <= n {
		return papers
	}

	// Build TF vectors for each paper
	vecs := make([]map[string]float64, len(papers))
	for i, p := range papers {
		vecs[i] = tfVector(p.Title + " " + p.Abstract)
	}

	// Greedy selection: pick the paper most different from already-selected
	selected := []int{0} // start with first paper
	used := map[int]bool{0: true}

	for len(selected) < n {
		bestIdx := -1
		bestDist := -1.0

		for i := range papers {
			if used[i] {
				continue
			}

			// Min distance to any already-selected paper
			minDist := math.MaxFloat64
			for _, si := range selected {
				d := cosineDist(vecs[i], vecs[si])
				if d < minDist {
					minDist = d
				}
			}

			if minDist > bestDist {
				bestDist = minDist
				bestIdx = i
			}
		}

		if bestIdx < 0 {
			break
		}
		selected = append(selected, bestIdx)
		used[bestIdx] = true
	}

	result := make([]Paper, 0, len(selected))
	for _, i := range selected {
		result = append(result, papers[i])
	}
	return result
}

// tfVector computes a simple term-frequency vector from text.
func tfVector(text string) map[string]float64 {
	words := strings.Fields(strings.ToLower(text))
	vec := make(map[string]float64)
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?()[]{}\"'")
		if len(w) < 3 {
			continue
		}
		vec[w]++
	}
	// Normalize
	total := 0.0
	for _, v := range vec {
		total += v * v
	}
	if total > 0 {
		norm := math.Sqrt(total)
		for k := range vec {
			vec[k] /= norm
		}
	}
	return vec
}

// cosineDist returns 1 - cosine_similarity between two vectors.
func cosineDist(a, b map[string]float64) float64 {
	dot := 0.0
	for k, v := range a {
		if bv, ok := b[k]; ok {
			dot += v * bv
		}
	}
	// Vectors are already normalized, so dot IS cosine similarity
	return 1.0 - dot
}
