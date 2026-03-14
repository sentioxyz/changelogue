package onboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/sentioxyz/changelogue/internal/queue"
)

// ScanStore is the data access interface for the scan worker.
type ScanStore interface {
	GetOnboardScan(ctx context.Context, id string) (*models.OnboardScan, error)
	UpdateOnboardScanStatus(ctx context.Context, id, status string, results json.RawMessage, scanErr string) error
}

// ScanWorker processes scan_dependencies River jobs.
type ScanWorker struct {
	river.WorkerDefaults[queue.ScanDependenciesJobArgs]
	store     ScanStore
	scanner   *Scanner
	extractor *DependencyExtractor
	pool      *pgxpool.Pool
}

// NewScanWorker creates a ScanWorker. extractor may be nil if GOOGLE_API_KEY is not set.
func NewScanWorker(store ScanStore, scanner *Scanner, extractor *DependencyExtractor, pool *pgxpool.Pool) *ScanWorker {
	return &ScanWorker{
		store:     store,
		scanner:   scanner,
		extractor: extractor,
		pool:      pool,
	}
}

func (w *ScanWorker) Timeout(_ *river.Job[queue.ScanDependenciesJobArgs]) time.Duration {
	return 3 * time.Minute
}

func (w *ScanWorker) Work(ctx context.Context, job *river.Job[queue.ScanDependenciesJobArgs]) error {
	scanID := job.Args.ScanID
	slog.Info("scan worker picked up job", "scan_id", scanID, "attempt", job.Attempt)

	// Load scan
	scan, err := w.store.GetOnboardScan(ctx, scanID)
	if err != nil {
		return fmt.Errorf("load scan %s: %w", scanID, err)
	}

	// Mark processing
	if err := w.store.UpdateOnboardScanStatus(ctx, scanID, "processing", nil, ""); err != nil {
		return fmt.Errorf("update status to processing: %w", err)
	}

	// Parse repo URL
	owner, repo, err := ParseRepoURL(scan.RepoURL)
	if err != nil {
		w.store.UpdateOnboardScanStatus(ctx, scanID, "failed", nil, err.Error())
		return fmt.Errorf("parse repo URL: %w", err)
	}

	// Fetch dependency files
	files, err := w.scanner.FetchDependencyFiles(ctx, owner, repo)
	if err != nil {
		w.store.UpdateOnboardScanStatus(ctx, scanID, "failed", nil, err.Error())
		return fmt.Errorf("fetch dependency files: %w", err)
	}

	slog.Info("found dependency files", "scan_id", scanID, "count", len(files))

	if len(files) == 0 {
		// No dependency files — store empty results
		emptyResults, _ := json.Marshal([]models.ScannedDependency{})
		w.store.UpdateOnboardScanStatus(ctx, scanID, "completed", emptyResults, "")
		w.notifyScanComplete(ctx, scanID)
		return nil
	}

	// Extract dependencies via LLM
	if w.extractor == nil {
		w.store.UpdateOnboardScanStatus(ctx, scanID, "failed", nil, "LLM not configured (set GOOGLE_API_KEY)")
		return river.JobCancel(fmt.Errorf("extractor not configured"))
	}

	deps, err := w.extractor.Extract(ctx, files)
	if err != nil {
		w.store.UpdateOnboardScanStatus(ctx, scanID, "failed", nil, err.Error())
		return fmt.Errorf("extract dependencies: %w", err)
	}

	slog.Info("extracted dependencies", "scan_id", scanID, "count", len(deps))

	results, err := json.Marshal(deps)
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}

	if err := w.store.UpdateOnboardScanStatus(ctx, scanID, "completed", results, ""); err != nil {
		return fmt.Errorf("update scan completed: %w", err)
	}

	w.notifyScanComplete(ctx, scanID)
	return nil
}

// notifyScanComplete sends a pg_notify on the release_events channel.
func (w *ScanWorker) notifyScanComplete(ctx context.Context, scanID string) {
	payload := fmt.Sprintf(`{"type":"scan_complete","id":"%s"}`, scanID)
	_, err := w.pool.Exec(ctx, "SELECT pg_notify('release_events', $1)", payload)
	if err != nil {
		slog.Error("pg_notify failed", "scan_id", scanID, "err", err)
	}
}
