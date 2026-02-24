package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

const schema = `
-- Drop old-format tables (dev only — no production data exists)
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS releases CASCADE;

-- Tracked software projects (the central entity)
CREATE TABLE IF NOT EXISTS projects (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    url VARCHAR(500),
    pipeline_config JSONB NOT NULL DEFAULT '{"changelog_summarizer": {}, "urgency_scorer": {}}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Configured ingestion sources
CREATE TABLE IF NOT EXISTS sources (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_type VARCHAR(50) NOT NULL,
    repository VARCHAR(255) NOT NULL,
    poll_interval_seconds INT DEFAULT 300,
    enabled BOOLEAN DEFAULT true,
    exclude_version_regexp TEXT,
    exclude_prereleases BOOLEAN DEFAULT false,
    last_polled_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(source_type, repository)
);

-- Normalized release events (references source, not raw strings)
CREATE TABLE IF NOT EXISTS releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id INT NOT NULL REFERENCES sources(id),
    version VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(source_id, version)
);

-- Pipeline job tracking (application-level, separate from River internals)
CREATE TABLE IF NOT EXISTS pipeline_jobs (
    id BIGSERIAL PRIMARY KEY,
    state VARCHAR(50) DEFAULT 'available',
    release_id UUID REFERENCES releases(id),
    current_node VARCHAR(50),
    node_results JSONB DEFAULT '{}',
    attempt INT DEFAULT 0,
    max_attempts INT DEFAULT 3,
    error_message TEXT,
    locked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Registered notification channels
CREATE TABLE IF NOT EXISTS notification_channels (
    id SERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Subscriptions: route project releases to notification channels
CREATE TABLE IF NOT EXISTS subscriptions (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    channel_type VARCHAR(50) NOT NULL,
    channel_id INT REFERENCES notification_channels(id),
    version_pattern TEXT,
    frequency VARCHAR(20) DEFAULT 'instant',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- API authentication keys
CREATE TABLE IF NOT EXISTS api_keys (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(12) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

-- Trigger for SSE: notify on new releases
CREATE OR REPLACE FUNCTION notify_release_created() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('release_events', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS release_created_trigger ON releases;
CREATE TRIGGER release_created_trigger
    AFTER INSERT ON releases
    FOR EACH ROW EXECUTE FUNCTION notify_release_created();
`

// RunMigrations applies River's schema and the application schema. Idempotent — safe to call on every startup.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrations: %w", err)
	}

	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("app migrations: %w", err)
	}
	return nil
}
