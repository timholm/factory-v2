package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/timholm/factory-v2/internal/discover"
	"github.com/timholm/factory-v2/internal/synthesize"
)

// Store wraps a Postgres connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New connects to Postgres and returns a Store.
func New(ctx context.Context, connStr string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close closes the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// Migrate creates tables if they don't exist.
func (s *Store) Migrate(ctx context.Context) error {
	ddl := `
	CREATE TABLE IF NOT EXISTS clusters (
		id SERIAL PRIMARY KEY,
		problem_space TEXT NOT NULL,
		paper_ids TEXT NOT NULL,
		techniques TEXT,
		score FLOAT DEFAULT 0,
		status TEXT DEFAULT 'pending',
		spec_json TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS builds (
		id SERIAL PRIMARY KEY,
		cluster_id INT REFERENCES clusters(id),
		name TEXT NOT NULL,
		language TEXT,
		status TEXT DEFAULT 'queued',
		github_url TEXT,
		quality_score INT,
		error_log TEXT,
		started_at TIMESTAMP,
		shipped_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS audit_scores (
		repo_name TEXT PRIMARY KEY,
		score INT,
		has_tests BOOLEAN,
		has_readme BOOLEAN,
		has_references BOOLEAN,
		correct_module_path BOOLEAN,
		compiles BOOLEAN,
		tests_pass BOOLEAN,
		audited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := s.pool.Exec(ctx, ddl)
	return err
}

// InsertCluster persists a cluster and returns its ID.
func (s *Store) InsertCluster(ctx context.Context, c discover.Cluster) (int, error) {
	paperJSON, _ := json.Marshal(c.PaperIDs)
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO clusters (problem_space, paper_ids, status) VALUES ($1, $2, 'pending') RETURNING id`,
		c.ProblemSpace, string(paperJSON),
	).Scan(&id)
	return id, err
}

// UpdateClusterStatus updates the status of a cluster.
func (s *Store) UpdateClusterStatus(ctx context.Context, id int, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE clusters SET status = $1 WHERE id = $2`, status, id)
	return err
}

// SaveSpec stores the spec JSON on a cluster row.
func (s *Store) SaveSpec(ctx context.Context, clusterID int, spec *synthesize.ProductSpec) error {
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE clusters SET spec_json = $1, status = 'synthesized' WHERE id = $2`,
		string(specJSON), clusterID)
	return err
}

// InsertBuild creates a build record.
func (s *Store) InsertBuild(ctx context.Context, clusterID int, spec *synthesize.ProductSpec) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO builds (cluster_id, name, language, status, started_at) VALUES ($1, $2, $3, 'building', $4) RETURNING id`,
		clusterID, spec.Name, spec.Language, time.Now(),
	).Scan(&id)
	return id, err
}

// UpdateBuildStatus updates build status and optional error log.
func (s *Store) UpdateBuildStatus(ctx context.Context, id int, status, errLog string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE builds SET status = $1, error_log = $2 WHERE id = $3`,
		status, errLog, id)
	return err
}

// UpdateBuildShipped marks a build as shipped.
func (s *Store) UpdateBuildShipped(ctx context.Context, id int, githubURL string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE builds SET status = 'shipped', github_url = $1, shipped_at = $2 WHERE id = $3`,
		githubURL, time.Now(), id)
	return err
}

// UpdateBuildScore sets the quality score.
func (s *Store) UpdateBuildScore(ctx context.Context, id int, score int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE builds SET quality_score = $1 WHERE id = $2`,
		score, id)
	return err
}

// UpsertAuditScore stores an audit score.
func (s *Store) UpsertAuditScore(ctx context.Context, a AuditScore) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_scores (repo_name, score, has_tests, has_readme, has_references, correct_module_path, compiles, tests_pass, audited_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (repo_name) DO UPDATE SET
		   score = $2, has_tests = $3, has_readme = $4, has_references = $5,
		   correct_module_path = $6, compiles = $7, tests_pass = $8, audited_at = $9`,
		a.RepoName, a.Score, a.HasTests, a.HasReadme, a.HasReferences,
		a.CorrectModulePath, a.Compiles, a.TestsPass, time.Now(),
	)
	return err
}

// AuditScore represents a row in audit_scores.
type AuditScore struct {
	RepoName          string
	Score             int
	HasTests          bool
	HasReadme         bool
	HasReferences     bool
	CorrectModulePath bool
	Compiles          bool
	TestsPass         bool
}

// PipelineStatus holds summary counts for reporting.
type PipelineStatus struct {
	TotalClusters int
	Pending       int
	Building      int
	Shipped       int
	Failed        int
	TotalBuilds   int
	AvgScore      float64
}

// GetPipelineStatus returns aggregate pipeline stats.
func (s *Store) GetPipelineStatus(ctx context.Context) (*PipelineStatus, error) {
	ps := &PipelineStatus{}

	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM clusters`).Scan(&ps.TotalClusters)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `SELECT status, COUNT(*) FROM clusters GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		switch status {
		case "pending":
			ps.Pending = count
		case "building":
			ps.Building = count
		case "shipped":
			ps.Shipped = count
		case "failed":
			ps.Failed = count
		}
	}

	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM builds`).Scan(&ps.TotalBuilds)
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(AVG(quality_score), 0) FROM builds WHERE quality_score IS NOT NULL`).Scan(&ps.AvgScore)

	return ps, nil
}

// ShippedRepoNames returns names of all shipped builds.
func (s *Store) ShippedRepoNames(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT name FROM builds WHERE status = 'shipped'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// RecentBuilds returns the most recent N builds.
type BuildRow struct {
	ID           int
	Name         string
	Language     string
	Status       string
	GitHubURL    string
	QualityScore *int
	ErrorLog     string
	CreatedAt    time.Time
}

func (s *Store) RecentBuilds(ctx context.Context, limit int) ([]BuildRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, COALESCE(language,''), status, COALESCE(github_url,''), quality_score, COALESCE(error_log,''), created_at
		 FROM builds ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var builds []BuildRow
	for rows.Next() {
		var b BuildRow
		if err := rows.Scan(&b.ID, &b.Name, &b.Language, &b.Status, &b.GitHubURL, &b.QualityScore, &b.ErrorLog, &b.CreatedAt); err != nil {
			continue
		}
		builds = append(builds, b)
	}
	return builds, nil
}
