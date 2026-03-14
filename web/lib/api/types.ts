// web/lib/api/types.ts

// --- Response Envelope ---

export interface ApiResponse<T> {
  data: T;
  meta?: ApiMeta;
  error?: ApiError;
}

export interface ApiMeta {
  page: number;
  per_page: number;
  total: number;
}

export interface ApiError {
  code: string;
  message: string;
}

// --- Domain Models (all IDs are UUID strings) ---

export interface Project {
  id: string;
  name: string;
  description?: string;
  agent_prompt?: string;
  agent_rules?: AgentRules;
  created_at: string;
  updated_at: string;
}

export interface AgentRules {
  on_major_release?: boolean;
  on_minor_release?: boolean;
  on_security_patch?: boolean;
  version_pattern?: string;
}

export interface ProjectInput {
  name: string;
  description?: string;
  agent_prompt?: string;
  agent_rules?: AgentRules;
}

export interface Source {
  id: string;
  project_id: string;
  provider: string;
  repository: string;
  poll_interval_seconds: number;
  enabled: boolean;
  config?: Record<string, unknown>;
  version_filter_include?: string;
  version_filter_exclude?: string;
  exclude_prereleases?: boolean;
  last_polled_at?: string;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface SourceInput {
  provider: string;
  repository: string;
  poll_interval_seconds: number;
  enabled: boolean;
  config?: Record<string, unknown>;
  version_filter_include?: string;
  version_filter_exclude?: string;
  exclude_prereleases?: boolean;
}

export interface Release {
  id: string;
  source_id: string;
  version: string;
  raw_data?: Record<string, unknown>;
  released_at?: string;
  created_at: string;
  project_id?: string;
  project_name?: string;
  provider?: string;
  repository?: string;
  excluded?: boolean;
  semantic_release_id?: string;
  semantic_release_status?: string;
  semantic_release_urgency?: string;
}

export interface ContextSource {
  id: string;
  project_id: string;
  type: string;
  name: string;
  config: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface ContextSourceInput {
  type: string;
  name: string;
  config: Record<string, unknown>;
}

export interface SemanticRelease {
  id: string;
  project_id: string;
  project_name?: string;
  version: string;
  report?: SemanticReport;
  status: string;
  error?: string;
  created_at: string;
  completed_at?: string;
}

export interface SemanticReport {
  subject?: string;
  urgency?: string;
  urgency_reason?: string;
  status_checks?: string[];
  changelog_summary?: string;
  download_commands?: string[];
  download_links?: string[];
  summary?: string;
  availability?: string;
  adoption?: string;
  recommendation?: string;
  // Backward compat — old reports may still have these
  risk_level?: string;
  risk_reason?: string;
}

export interface AgentRun {
  id: string;
  project_id: string;
  semantic_release_id?: string;
  trigger: string;
  status: string;
  prompt_used?: string;
  error?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
}

export interface NotificationChannel {
  id: string;
  name: string;
  type: string;
  config: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface ChannelInput {
  type: string;
  name: string;
  config: Record<string, unknown>;
}

export interface Subscription {
  id: string;
  channel_id: string;
  type: "source_release" | "semantic_release";
  source_id?: string;
  project_id?: string;
  version_filter?: string;
  created_at: string;
}

export interface SubscriptionInput {
  channel_id: string;
  type: "source_release" | "semantic_release";
  source_id?: string;
  project_id?: string;
  version_filter?: string;
}

export interface BatchSubscriptionInput {
  channel_id: string;
  type: "source_release" | "semantic_release";
  project_ids?: string[];
  source_ids?: string[];
  version_filter?: string;
}

export interface BatchDeleteSubscriptionInput {
  ids: string[];
}

// --- Todo Types ---

export interface Todo {
  id: string;
  release_id?: string;
  semantic_release_id?: string;
  status: "pending" | "acknowledged" | "resolved";
  created_at: string;
  acknowledged_at?: string;
  resolved_at?: string;
  project_id?: string;
  project_name?: string;
  version?: string;
  provider?: string;
  repository?: string;
  source_url?: string;
  release_url?: string;
  urgency?: string;
  todo_type?: "release" | "semantic";
}

// --- System Types ---

export interface HealthStatus {
  status: string;
  database: string;
}

export interface Stats {
  total_projects: number;
  active_sources: number;
  total_releases: number;
  pending_agent_runs: number;
  releases_this_week: number;
  attention_needed: number;
}

export interface TrendBucket {
  period: string;
  releases: number;
  semantic_releases: number;
}

export interface TrendData {
  granularity: string;
  buckets: TrendBucket[];
}

// --- Discovery Types ---

export interface DiscoverItem {
  name: string;
  full_name: string;
  description: string;
  stars: number;
  language?: string;
  url: string;
  avatar_url?: string;
  provider: "github" | "dockerhub";
}

// --- SSE Event Types ---

export type SSEEventType = "release" | "semantic_release";

interface SSEBase {
  id: string;
  timestamp: string;
}

export type SSEEvent =
  | (SSEBase & {
      type: "release";
      data: { id: string; source_id: string; version: string; created_at: string };
    })
  | (SSEBase & {
      type: "semantic_release";
      data: { id: string; project_id: string; version: string; status: string };
    });

// --- Onboarding Types ---

export interface OnboardScan {
  id: string;
  repo_url: string;
  status: "pending" | "processing" | "completed" | "failed";
  results?: ScannedDependency[];
  error?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface ScannedDependency {
  name: string;
  version: string;
  ecosystem: string;
  upstream_repo: string;
  provider: string;
}

export interface OnboardSelection {
  dep_name: string;
  upstream_repo: string;
  provider: string;
  project_id?: string;
  new_project_name?: string;
}

export interface OnboardApplyResult {
  created_projects: Project[];
  created_sources: Source[];
  skipped: string[];
}
