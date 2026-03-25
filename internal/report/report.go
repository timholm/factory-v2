package report

import (
	"context"
	"fmt"
	"log"

	"github.com/timholm/factory-v2/internal/db"
)

// Reporter generates pipeline reports.
type Reporter struct {
	db *db.Store
}

// New creates a Reporter.
func New(store *db.Store) *Reporter {
	return &Reporter{db: store}
}

// Generate produces and logs a daily report.
func (r *Reporter) Generate(ctx context.Context) error {
	status, err := r.db.GetPipelineStatus(ctx)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	builds, err := r.db.RecentBuilds(ctx, 10)
	if err != nil {
		return fmt.Errorf("get recent builds: %w", err)
	}

	log.Println("=== Factory v2 Report ===")
	log.Printf("  Clusters: %d total (%d pending, %d building, %d shipped, %d failed)",
		status.TotalClusters, status.Pending, status.Building, status.Shipped, status.Failed)
	log.Printf("  Builds: %d total, avg quality: %.1f", status.TotalBuilds, status.AvgScore)

	if status.TotalBuilds > 0 {
		shipRate := float64(status.Shipped) / float64(status.TotalClusters) * 100
		log.Printf("  Ship rate: %.1f%%", shipRate)
	}

	log.Println("  Recent builds:")
	for _, b := range builds {
		scoreStr := "n/a"
		if b.QualityScore != nil {
			scoreStr = fmt.Sprintf("%d", *b.QualityScore)
		}
		log.Printf("    [%s] %s (%s) score=%s %s", b.Status, b.Name, b.Language, scoreStr, b.GitHubURL)
	}

	log.Println("=========================")
	return nil
}

// PrintStatus prints a summary to stdout (for the status command).
func PrintStatus(ctx context.Context, store *db.Store) error {
	status, err := store.GetPipelineStatus(ctx)
	if err != nil {
		return err
	}

	fmt.Println("Factory v2 Pipeline Status")
	fmt.Println("==========================")
	fmt.Printf("Clusters: %d total\n", status.TotalClusters)
	fmt.Printf("  Pending:  %d\n", status.Pending)
	fmt.Printf("  Building: %d\n", status.Building)
	fmt.Printf("  Shipped:  %d\n", status.Shipped)
	fmt.Printf("  Failed:   %d\n", status.Failed)
	fmt.Println()
	fmt.Printf("Builds: %d total\n", status.TotalBuilds)
	fmt.Printf("  Avg Quality: %.1f/100\n", status.AvgScore)

	if status.TotalClusters > 0 {
		shipRate := float64(status.Shipped) / float64(status.TotalClusters) * 100
		fmt.Printf("  Ship Rate:   %.1f%%\n", shipRate)
	}

	builds, err := store.RecentBuilds(ctx, 10)
	if err != nil {
		return err
	}

	if len(builds) > 0 {
		fmt.Println("\nRecent Builds:")
		for _, b := range builds {
			scoreStr := "n/a"
			if b.QualityScore != nil {
				scoreStr = fmt.Sprintf("%d", *b.QualityScore)
			}
			fmt.Printf("  [%-8s] %-30s %-6s score=%-4s %s\n",
				b.Status, b.Name, b.Language, scoreStr, b.GitHubURL)
		}
	}

	return nil
}
