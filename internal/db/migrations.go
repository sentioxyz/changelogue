package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

const schema = `
-- Tracked software projects (the central entity)
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    agent_prompt TEXT,
    agent_rules JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Configured ingestion sources (polling-based: GitHub, Docker Hub)
CREATE TABLE IF NOT EXISTS sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    repository VARCHAR(255) NOT NULL,
    poll_interval_seconds INT DEFAULT 86400,
    enabled BOOLEAN DEFAULT true,
    config JSONB,
    last_polled_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(provider, repository)
);

-- Context sources (read-only references for agent research)
CREATE TABLE IF NOT EXISTS context_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    config JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Source-level releases (detected from polling sources)
CREATE TABLE IF NOT EXISTS releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    raw_data JSONB,
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(source_id, version)
);

-- Project-level semantic releases (AI-generated, correlating source releases)
CREATE TABLE IF NOT EXISTS semantic_releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    report JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE(project_id, version)
);

-- Join table: which source releases compose a semantic release
CREATE TABLE IF NOT EXISTS semantic_release_sources (
    semantic_release_id UUID NOT NULL REFERENCES semantic_releases(id) ON DELETE CASCADE,
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    PRIMARY KEY (semantic_release_id, release_id)
);

-- Notification channels (standalone)
CREATE TABLE IF NOT EXISTS notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Subscriptions: two types (source releases and semantic releases)
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('source_release', 'semantic_release')),
    source_id UUID REFERENCES sources(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    version_filter TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CHECK (
        (type = 'source_release'   AND source_id  IS NOT NULL AND project_id IS NULL) OR
        (type = 'semantic_release' AND project_id IS NOT NULL AND source_id  IS NULL)
    )
);

-- Agent runs (scoped to project)
CREATE TABLE IF NOT EXISTS agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    semantic_release_id UUID REFERENCES semantic_releases(id) ON DELETE SET NULL,
    trigger VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    prompt_used TEXT,
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS version VARCHAR(100);

-- API authentication keys
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(12) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

-- Trigger for SSE: notify on new releases
CREATE OR REPLACE FUNCTION notify_release_created() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('release_events', json_build_object('type', 'release', 'id', NEW.id)::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS release_created_trigger ON releases;
CREATE TRIGGER release_created_trigger
    AFTER INSERT ON releases
    FOR EACH ROW EXECUTE FUNCTION notify_release_created();

-- Trigger for SSE: notify on semantic release completion
CREATE OR REPLACE FUNCTION notify_semantic_release() RETURNS trigger AS $$
BEGIN
    IF NEW.status = 'completed' AND (OLD IS NULL OR OLD.status != 'completed') THEN
        PERFORM pg_notify('release_events', json_build_object('type', 'semantic_release', 'id', NEW.id)::text);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS semantic_release_trigger ON semantic_releases;
CREATE TRIGGER semantic_release_trigger
    AFTER INSERT OR UPDATE ON semantic_releases
    FOR EACH ROW EXECUTE FUNCTION notify_semantic_release();

-- Release TODOs (acknowledge/resolve tracking)
CREATE TABLE IF NOT EXISTS release_todos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id UUID REFERENCES releases(id) ON DELETE CASCADE,
    semantic_release_id UUID REFERENCES semantic_releases(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    CHECK (
        (release_id IS NOT NULL AND semantic_release_id IS NULL) OR
        (release_id IS NULL AND semantic_release_id IS NOT NULL)
    )
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_release_todos_release_id ON release_todos(release_id) WHERE release_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_release_todos_semantic_release_id ON release_todos(semantic_release_id) WHERE semantic_release_id IS NOT NULL;

-- Onboarding scans (GitHub repo dependency detection)
CREATE TABLE IF NOT EXISTS onboard_scans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_url TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    results JSONB,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

-- Authenticated users (GitHub OAuth)
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id       BIGINT NOT NULL UNIQUE,
    github_login    VARCHAR(100) NOT NULL,
    name            VARCHAR(200),
    avatar_url      TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- User sessions (server-side, referenced by HttpOnly cookie)
CREATE TABLE IF NOT EXISTS sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- Release gates (per-project gate configuration)
CREATE TABLE IF NOT EXISTS release_gates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    required_sources JSONB,
    timeout_hours INT NOT NULL DEFAULT 168,
    version_mapping JSONB,
    nl_rule TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Version readiness (per-version gate state tracking)
CREATE TABLE IF NOT EXISTS version_readiness (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'ready', 'timed_out')),
    sources_met JSONB NOT NULL DEFAULT '[]',
    sources_missing JSONB NOT NULL DEFAULT '[]',
    nl_rule_passed BOOLEAN,
    timeout_at TIMESTAMPTZ NOT NULL,
    opened_at TIMESTAMPTZ,
    agent_triggered BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, version)
);
CREATE INDEX IF NOT EXISTS idx_version_readiness_timeout
    ON version_readiness(timeout_at) WHERE status = 'pending';

-- Gate events (audit log for gate lifecycle)
CREATE TABLE IF NOT EXISTS gate_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_readiness_id UUID NOT NULL REFERENCES version_readiness(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version VARCHAR(100) NOT NULL,
    event_type VARCHAR(30) NOT NULL,
    source_id UUID,
    details JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gate_events_readiness ON gate_events(version_readiness_id, created_at);
CREATE INDEX IF NOT EXISTS idx_gate_events_project ON gate_events(project_id, created_at);
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

	// Migrate existing subscription type values: source→source_release, project→semantic_release.
	// Drop old CHECK constraints first (both the named type check and the unnamed
	// composite check referencing old values), then UPDATE data, then re-add constraints.
	if _, err := pool.Exec(ctx, `
		ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_type_check;
		ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_check;
		UPDATE subscriptions SET type = 'source_release' WHERE type = 'source';
		UPDATE subscriptions SET type = 'semantic_release' WHERE type = 'project';
		ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_type_check CHECK (type IN ('source_release', 'semantic_release'));
		ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_check CHECK (
			(type = 'source_release'   AND source_id IS NOT NULL AND project_id IS NULL) OR
			(type = 'semantic_release' AND project_id IS NOT NULL AND source_id IS NULL)
		);
	`); err != nil {
		return fmt.Errorf("subscription type migration: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		ALTER TABLE sources ADD COLUMN IF NOT EXISTS version_filter_include TEXT;
		ALTER TABLE sources ADD COLUMN IF NOT EXISTS version_filter_exclude TEXT;
	`); err != nil {
		return fmt.Errorf("source version filter migration: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		ALTER TABLE sources ADD COLUMN IF NOT EXISTS exclude_prereleases BOOLEAN DEFAULT false;
	`); err != nil {
		return fmt.Errorf("source exclude_prereleases migration: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS config JSONB;
	`); err != nil {
		return fmt.Errorf("subscription config migration: %w", err)
	}

	// Auto-create release_gates for projects with WaitForAllSources enabled.
	if _, err := pool.Exec(ctx, `
		INSERT INTO release_gates (project_id, timeout_hours, enabled)
		SELECT p.id, 168, true
		FROM projects p
		WHERE p.agent_rules->>'wait_for_all_sources' = 'true'
		ON CONFLICT (project_id) DO NOTHING
	`); err != nil {
		return fmt.Errorf("wait_for_all_sources migration: %w", err)
	}

	// Backfill semantic_release_sources for existing semantic releases that were
	// created with normalized versions (e.g., gate stripped v-prefix).
	if _, err := pool.Exec(ctx, `
		INSERT INTO semantic_release_sources (semantic_release_id, release_id)
		SELECT sr.id, r.id
		FROM semantic_releases sr
		JOIN sources s ON s.project_id = sr.project_id
		JOIN releases r ON r.source_id = s.id
		  AND LOWER(TRIM(LEADING 'v' FROM TRIM(LEADING 'V' FROM r.version)))
		    = LOWER(TRIM(LEADING 'v' FROM TRIM(LEADING 'V' FROM sr.version)))
		WHERE NOT EXISTS (
		    SELECT 1 FROM semantic_release_sources srs
		    WHERE srs.semantic_release_id = sr.id AND srs.release_id = r.id
		)
		ON CONFLICT DO NOTHING
	`); err != nil {
		return fmt.Errorf("backfill semantic_release_sources: %w", err)
	}

	return nil
}
