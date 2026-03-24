package queue

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// NewRiverClient creates a River client backed by the given pgx pool.
// Pass nil workers to create an insert-only client (no job processing).
// Optional periodicJobs are registered on the client when workers are present.
func NewRiverClient(pool *pgxpool.Pool, workers *river.Workers, periodicJobs ...*river.PeriodicJob) (*river.Client[pgx.Tx], error) {
	config := &river.Config{}
	if workers != nil {
		config.Workers = workers
		config.Queues = map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 100},
		}
	}
	if len(periodicJobs) > 0 {
		config.PeriodicJobs = periodicJobs
	}
	return river.NewClient(riverpgxv5.New(pool), config)
}
