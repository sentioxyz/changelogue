// web/lib/api/types.ts

// --- Response Envelope ---

export interface ApiMeta {
  request_id: string;
  page?: number;
  per_page?: number;
  total?: number;
}

export interface ApiResponse<T> {
  data: T;
  meta: ApiMeta;
}

export interface ApiError {
  error: {
    code: string;
    message: string;
  };
  meta: ApiMeta;
}

// --- Domain Models ---

export interface SemanticVersion {
  major: number;
  minor: number;
  patch: number;
  pre_release: string;
}

export interface Project {
  id: number;
  name: string;
  description: string;
  url: string;
  pipeline_config: Record<string, unknown>;
  sources?: Source[];
  subscription_count?: number;
  created_at: string;
  updated_at: string;
}

export interface ProjectInput {
  name: string;
  description: string;
  url: string;
  pipeline_config?: Record<string, unknown>;
}

export interface Release {
  id: string;
  source_id: number;
  source_type: string;
  repository: string;
  project_id: number;
  project_name: string;
  raw_version: string;
  semantic_version: SemanticVersion;
  is_pre_release: boolean;
  metadata: Record<string, string>;
  pipeline_status: "available" | "running" | "completed" | "retry" | "discarded";
  created_at: string;
}

export interface PipelineStatus {
  release_id: string;
  state: "available" | "running" | "completed" | "retry" | "discarded";
  current_node: string | null;
  node_results: Record<string, unknown>;
  attempt: number;
  completed_at: string | null;
}

export interface Source {
  id: number;
  project_id: number;
  type: string;
  repository: string;
  poll_interval_seconds: number;
  enabled: boolean;
  exclude_version_regexp: string;
  exclude_prereleases: boolean;
  last_polled_at: string | null;
  last_error: string | null;
  created_at: string;
  updated_at: string;
}

export interface SourceInput {
  project_id: number;
  type: string;
  repository: string;
  poll_interval_seconds: number;
  enabled: boolean;
  exclude_version_regexp?: string;
  exclude_prereleases?: boolean;
}

export interface Subscription {
  id: number;
  project_id: number;
  channel_type: string;
  channel_id: number;
  version_pattern: string;
  frequency: "instant" | "hourly" | "daily" | "weekly";
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface SubscriptionInput {
  project_id: number;
  channel_type: string;
  channel_id: number;
  version_pattern?: string;
  frequency?: "instant" | "hourly" | "daily" | "weekly";
  enabled?: boolean;
}

export interface NotificationChannel {
  id: number;
  type: string;
  name: string;
  config: Record<string, string>;
  enabled: boolean;
  created_at: string;
}

export interface ChannelInput {
  type: string;
  name: string;
  config: Record<string, string>;
  enabled?: boolean;
}

export interface Provider {
  type: string;
  name: string;
  description: string;
}

export interface HealthStatus {
  status: string;
  checks: Record<string, string>;
}

export interface Stats {
  total_releases: number;
  active_sources: number;
  pending_jobs: number;
  failed_jobs: number;
}

// --- SSE Event Types ---

export type SSEEventType =
  | "release.created"
  | "pipeline.node_completed"
  | "pipeline.completed"
  | "pipeline.failed"
  | "source.error"
  | "source.polled";

interface SSEBase {
  id: string;
  timestamp: string;
}

export type SSEEvent =
  | (SSEBase & {
      type: "release.created";
      data: { id: string; source: string; repository: string; raw_version: string; created_at: string };
    })
  | (SSEBase & {
      type: "pipeline.node_completed";
      data: { release_id: string; node: string; result: Record<string, unknown> };
    })
  | (SSEBase & {
      type: "pipeline.completed";
      data: { release_id: string; state: string };
    })
  | (SSEBase & {
      type: "pipeline.failed";
      data: { release_id: string };
    })
  | (SSEBase & {
      type: "source.polled";
      data: { source_id: number; repository: string; new_releases: number };
    })
  | (SSEBase & {
      type: "source.error";
      data: { repository: string };
    });
