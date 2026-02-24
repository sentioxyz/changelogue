// web/lib/mock/data.ts
import type {
  Project,
  Release,
  PipelineStatus,
  Source,
  Subscription,
  NotificationChannel,
  Provider,
  Stats,
} from "../api/types";

// --- Projects ---

export const mockProjects: Project[] = [
  {
    id: 1,
    name: "Geth",
    description: "Go Ethereum - Official Go implementation of the Ethereum protocol",
    url: "https://geth.ethereum.org",
    pipeline_config: {
      availability_checker: { extra_artifacts: [{ type: "npm_package", name: "geth" }] },
      risk_assessor: { keywords: ["hard fork", "breaking change", "CVE"] },
      adoption_tracker: { provider: "ethernodes", config: { network: "mainnet" } },
      changelog_summarizer: {},
      urgency_scorer: {},
    },
    subscription_count: 3,
    created_at: "2026-01-15T10:00:00Z",
    updated_at: "2026-02-20T14:30:00Z",
  },
  {
    id: 2,
    name: "Go",
    description: "The Go programming language",
    url: "https://go.dev",
    pipeline_config: { changelog_summarizer: {}, urgency_scorer: {} },
    subscription_count: 2,
    created_at: "2026-01-20T08:00:00Z",
    updated_at: "2026-02-18T09:00:00Z",
  },
  {
    id: 3,
    name: "PostgreSQL",
    description: "The world's most advanced open source relational database",
    url: "https://postgresql.org",
    pipeline_config: {
      risk_assessor: { keywords: ["CVE", "security fix", "data corruption"] },
      changelog_summarizer: {},
      urgency_scorer: {},
    },
    subscription_count: 1,
    created_at: "2026-02-01T12:00:00Z",
    updated_at: "2026-02-22T16:00:00Z",
  },
  {
    id: 4,
    name: "Node.js",
    description: "Node.js JavaScript runtime",
    url: "https://nodejs.org",
    pipeline_config: { changelog_summarizer: {}, urgency_scorer: {} },
    subscription_count: 2,
    created_at: "2026-02-10T11:00:00Z",
    updated_at: "2026-02-23T10:00:00Z",
  },
];

// --- Sources ---

export const mockSources: Source[] = [
  {
    id: 1, project_id: 1, type: "dockerhub", repository: "ethereum/client-go",
    poll_interval_seconds: 300, enabled: true, exclude_version_regexp: "",
    exclude_prereleases: false, last_polled_at: "2026-02-24T10:25:00Z",
    last_error: null, created_at: "2026-01-15T10:00:00Z", updated_at: "2026-02-24T10:25:00Z",
  },
  {
    id: 2, project_id: 1, type: "github", repository: "ethereum/go-ethereum",
    poll_interval_seconds: 600, enabled: true, exclude_version_regexp: "-(alpha|nightly)",
    exclude_prereleases: true, last_polled_at: "2026-02-24T10:20:00Z",
    last_error: null, created_at: "2026-01-15T10:05:00Z", updated_at: "2026-02-24T10:20:00Z",
  },
  {
    id: 3, project_id: 2, type: "dockerhub", repository: "library/golang",
    poll_interval_seconds: 300, enabled: true, exclude_version_regexp: "",
    exclude_prereleases: false, last_polled_at: "2026-02-24T10:28:00Z",
    last_error: null, created_at: "2026-01-20T08:00:00Z", updated_at: "2026-02-24T10:28:00Z",
  },
  {
    id: 4, project_id: 2, type: "github", repository: "golang/go",
    poll_interval_seconds: 600, enabled: true, exclude_version_regexp: "",
    exclude_prereleases: true, last_polled_at: "2026-02-24T10:15:00Z",
    last_error: null, created_at: "2026-01-20T08:05:00Z", updated_at: "2026-02-24T10:15:00Z",
  },
  {
    id: 5, project_id: 3, type: "dockerhub", repository: "library/postgres",
    poll_interval_seconds: 600, enabled: true, exclude_version_regexp: "-(beta|rc)",
    exclude_prereleases: true, last_polled_at: "2026-02-24T10:10:00Z",
    last_error: null, created_at: "2026-02-01T12:00:00Z", updated_at: "2026-02-24T10:10:00Z",
  },
  {
    id: 6, project_id: 4, type: "github", repository: "nodejs/node",
    poll_interval_seconds: 300, enabled: true, exclude_version_regexp: "",
    exclude_prereleases: false, last_polled_at: "2026-02-24T10:30:00Z",
    last_error: "rate limited", created_at: "2026-02-10T11:00:00Z", updated_at: "2026-02-24T10:30:00Z",
  },
];

// --- Releases ---

export const mockReleases: Release[] = [
  {
    id: "r-001", source_id: 2, source_type: "github", repository: "ethereum/go-ethereum",
    project_id: 1, project_name: "Geth", raw_version: "v1.14.13",
    semantic_version: { major: 1, minor: 14, patch: 13, pre_release: "" },
    is_pre_release: false, metadata: { commit: "abc123" }, pipeline_status: "completed",
    created_at: "2026-02-24T09:00:00Z",
  },
  {
    id: "r-002", source_id: 1, source_type: "dockerhub", repository: "ethereum/client-go",
    project_id: 1, project_name: "Geth", raw_version: "v1.14.13",
    semantic_version: { major: 1, minor: 14, patch: 13, pre_release: "" },
    is_pre_release: false, metadata: { digest: "sha256:def456" }, pipeline_status: "completed",
    created_at: "2026-02-24T09:05:00Z",
  },
  {
    id: "r-003", source_id: 4, source_type: "github", repository: "golang/go",
    project_id: 2, project_name: "Go", raw_version: "go1.23.0",
    semantic_version: { major: 1, minor: 23, patch: 0, pre_release: "" },
    is_pre_release: false, metadata: {}, pipeline_status: "completed",
    created_at: "2026-02-23T15:00:00Z",
  },
  {
    id: "r-004", source_id: 3, source_type: "dockerhub", repository: "library/golang",
    project_id: 2, project_name: "Go", raw_version: "1.23.0",
    semantic_version: { major: 1, minor: 23, patch: 0, pre_release: "" },
    is_pre_release: false, metadata: { digest: "sha256:ghi789" }, pipeline_status: "running",
    created_at: "2026-02-23T15:30:00Z",
  },
  {
    id: "r-005", source_id: 5, source_type: "dockerhub", repository: "library/postgres",
    project_id: 3, project_name: "PostgreSQL", raw_version: "17.3",
    semantic_version: { major: 17, minor: 3, patch: 0, pre_release: "" },
    is_pre_release: false, metadata: { digest: "sha256:jkl012" }, pipeline_status: "completed",
    created_at: "2026-02-22T08:00:00Z",
  },
  {
    id: "r-006", source_id: 6, source_type: "github", repository: "nodejs/node",
    project_id: 4, project_name: "Node.js", raw_version: "v22.14.0",
    semantic_version: { major: 22, minor: 14, patch: 0, pre_release: "" },
    is_pre_release: false, metadata: {}, pipeline_status: "completed",
    created_at: "2026-02-21T12:00:00Z",
  },
  {
    id: "r-007", source_id: 6, source_type: "github", repository: "nodejs/node",
    project_id: 4, project_name: "Node.js", raw_version: "v23.0.0-rc.1",
    semantic_version: { major: 23, minor: 0, patch: 0, pre_release: "rc.1" },
    is_pre_release: true, metadata: {}, pipeline_status: "available",
    created_at: "2026-02-24T06:00:00Z",
  },
  {
    id: "r-008", source_id: 2, source_type: "github", repository: "ethereum/go-ethereum",
    project_id: 1, project_name: "Geth", raw_version: "v1.15.0-rc.1",
    semantic_version: { major: 1, minor: 15, patch: 0, pre_release: "rc.1" },
    is_pre_release: true, metadata: {}, pipeline_status: "discarded",
    created_at: "2026-02-24T08:00:00Z",
  },
];

// --- Pipeline Statuses ---

export const mockPipelineStatuses: Record<string, PipelineStatus> = {
  "r-001": {
    release_id: "r-001", state: "completed", current_node: null,
    node_results: {
      regex_normalizer: { semantic_version: { major: 1, minor: 14, patch: 13 }, is_pre_release: false },
      availability_checker: { docker_image: "verified", binaries: "available" },
      risk_assessor: { level: "critical", keywords: ["Hard Fork"], signal_source: "Discord #announcements" },
      adoption_tracker: { percentage: 12, recommendation: "Wait recommended if not urgent" },
      changelog_summarizer: { summary: "This release includes the Shanghai hard fork activation, EIP-4895 withdrawals, and critical sync fixes for block 14M." },
      urgency_scorer: { score: "high", factors: ["critical_risk_level", "low_adoption"] },
    },
    attempt: 1, completed_at: "2026-02-24T09:00:05Z",
  },
  "r-003": {
    release_id: "r-003", state: "completed", current_node: null,
    node_results: {
      regex_normalizer: { semantic_version: { major: 1, minor: 23, patch: 0 }, is_pre_release: false },
      changelog_summarizer: { summary: "Go 1.23 introduces range-over-func iterators, new unique package, and improved TLS defaults." },
      urgency_scorer: { score: "medium", factors: ["minor_version_bump"] },
    },
    attempt: 1, completed_at: "2026-02-23T15:00:03Z",
  },
  "r-004": {
    release_id: "r-004", state: "running", current_node: "changelog_summarizer",
    node_results: {
      regex_normalizer: { semantic_version: { major: 1, minor: 23, patch: 0 }, is_pre_release: false },
    },
    attempt: 1, completed_at: null,
  },
  "r-005": {
    release_id: "r-005", state: "completed", current_node: null,
    node_results: {
      regex_normalizer: { semantic_version: { major: 17, minor: 3, patch: 0 }, is_pre_release: false },
      risk_assessor: { level: "low", keywords: [] },
      changelog_summarizer: { summary: "PostgreSQL 17.3 includes minor bug fixes and performance improvements." },
      urgency_scorer: { score: "low", factors: ["patch_release", "no_risk_signals"] },
    },
    attempt: 1, completed_at: "2026-02-22T08:00:04Z",
  },
  "r-008": {
    release_id: "r-008", state: "discarded", current_node: "availability_checker",
    node_results: {
      regex_normalizer: { semantic_version: { major: 1, minor: 15, patch: 0 }, is_pre_release: true },
    },
    attempt: 3, completed_at: null,
  },
};

// --- Subscriptions ---

export const mockSubscriptions: Subscription[] = [
  {
    id: 1, project_id: 1, channel_type: "stable", channel_id: 1,
    version_pattern: "^v\\d+\\.\\d+\\.\\d+$", frequency: "instant", enabled: true,
    created_at: "2026-01-15T10:30:00Z", updated_at: "2026-02-20T14:30:00Z",
  },
  {
    id: 2, project_id: 1, channel_type: "security", channel_id: 2,
    version_pattern: "", frequency: "instant", enabled: true,
    created_at: "2026-01-15T10:35:00Z", updated_at: "2026-01-15T10:35:00Z",
  },
  {
    id: 3, project_id: 1, channel_type: "pre-release", channel_id: 1,
    version_pattern: "rc|alpha|beta", frequency: "daily", enabled: true,
    created_at: "2026-01-15T10:40:00Z", updated_at: "2026-01-15T10:40:00Z",
  },
  {
    id: 4, project_id: 2, channel_type: "stable", channel_id: 1,
    version_pattern: "", frequency: "instant", enabled: true,
    created_at: "2026-01-20T08:30:00Z", updated_at: "2026-01-20T08:30:00Z",
  },
  {
    id: 5, project_id: 2, channel_type: "stable", channel_id: 3,
    version_pattern: "", frequency: "weekly", enabled: true,
    created_at: "2026-01-20T08:35:00Z", updated_at: "2026-01-20T08:35:00Z",
  },
];

// --- Notification Channels ---

export const mockChannels: NotificationChannel[] = [
  {
    id: 1, type: "slack", name: "Engineering Releases",
    config: { webhook_url: "https://hooks.slack.com/services/T00/B00/xxx", channel: "#releases" },
    enabled: true, created_at: "2026-01-10T09:00:00Z",
  },
  {
    id: 2, type: "pagerduty", name: "Security Alerts",
    config: { routing_key: "R0xxxxx" },
    enabled: true, created_at: "2026-01-10T09:05:00Z",
  },
  {
    id: 3, type: "webhook", name: "Ops Webhook",
    config: { url: "https://ops.internal/api/releases", headers: "Authorization: Bearer tok123" },
    enabled: true, created_at: "2026-01-12T14:00:00Z",
  },
];

// --- Providers ---

export const mockProviders: Provider[] = [
  { type: "dockerhub", name: "Docker Hub", description: "Poll Docker Hub image tags" },
  { type: "github", name: "GitHub Releases", description: "Poll GitHub releases or receive webhooks" },
];

// --- Stats ---

export const mockStats: Stats = {
  total_releases: 847,
  active_sources: 6,
  pending_jobs: 1,
  failed_jobs: 1,
};
