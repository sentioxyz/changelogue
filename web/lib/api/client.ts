// web/lib/api/client.ts
import type {
  ApiResponse,
  Project,
  ProjectInput,
  Release,
  Source,
  SourceInput,
  Subscription,
  SubscriptionInput,
  BatchSubscriptionInput,
  BatchDeleteSubscriptionInput,
  NotificationChannel,
  ChannelInput,
  ContextSource,
  ContextSourceInput,
  SemanticRelease,
  AgentRun,
  HealthStatus,
  Stats,
  TrendData,
  DiscoverItem,
  Todo,
  OnboardScan,
  OnboardSelection,
  OnboardApplyResult,
  SuggestionItem,
  RepoItem,
  ReleaseGate,
  ReleaseGateInput,
  VersionReadiness,
  GateEvent,
  ApiKey,
  ApiKeyCreateInput,
} from "./types";

const BASE = process.env.NEXT_PUBLIC_API_URL || "/api/v1";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new Error(body?.error?.message ?? `Request failed: ${res.status}`);
  }
  if (res.status === 204) return null as T;
  return res.json();
}

// --- Projects ---

export const projects = {
  list: (page = 1, perPage = 25) =>
    request<ApiResponse<Project[]>>(`/projects?page=${page}&per_page=${perPage}`),
  get: (id: string) =>
    request<ApiResponse<Project>>(`/projects/${id}`),
  create: (input: ProjectInput) =>
    request<ApiResponse<Project>>("/projects", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: string, input: ProjectInput) =>
    request<ApiResponse<Project>>(`/projects/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (id: string) =>
    request<ApiResponse<null>>(`/projects/${id}`, { method: "DELETE" }),
};

// --- Sources (nested under projects) ---

export const sources = {
  listByProject: (projectId: string, page = 1) =>
    request<ApiResponse<Source[]>>(`/projects/${projectId}/sources?page=${page}`),
  create: (projectId: string, input: SourceInput) =>
    request<ApiResponse<Source>>(`/projects/${projectId}/sources`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  get: (id: string) =>
    request<ApiResponse<Source>>(`/sources/${id}`),
  update: (id: string, input: SourceInput) =>
    request<ApiResponse<Source>>(`/sources/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (id: string) =>
    request<ApiResponse<null>>(`/sources/${id}`, { method: "DELETE" }),
  poll: (id: string) =>
    request<ApiResponse<{ new_releases: number }>>(`/sources/${id}/poll`, {
      method: "POST",
    }),
};

export interface ReleaseFilters {
  provider?: string;
  urgency?: string;
  date_from?: string;
  date_to?: string;
  include_excluded?: boolean;
}

function buildReleaseParams(page: number, perPage: number, filters?: ReleaseFilters): string {
  const p = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (filters?.include_excluded) p.set("include_excluded", "true");
  if (filters?.provider) p.set("provider", filters.provider);
  if (filters?.urgency) p.set("urgency", filters.urgency);
  if (filters?.date_from) p.set("date_from", filters.date_from);
  if (filters?.date_to) p.set("date_to", filters.date_to);
  return p.toString();
}

export const releases = {
  list: (page = 1, perPage = 25, filters?: ReleaseFilters) =>
    request<ApiResponse<Release[]>>(`/releases?${buildReleaseParams(page, perPage, filters)}`),
  listBySource: (sourceId: string, page = 1, perPage = 25, filters?: ReleaseFilters) =>
    request<ApiResponse<Release[]>>(`/sources/${sourceId}/releases?${buildReleaseParams(page, perPage, filters)}`),
  listByProject: (projectId: string, page = 1, perPage = 25, filters?: ReleaseFilters) =>
    request<ApiResponse<Release[]>>(`/projects/${projectId}/releases?${buildReleaseParams(page, perPage, filters)}`),
  get: (id: string) =>
    request<ApiResponse<Release>>(`/releases/${id}`),
};

// --- Context Sources ---

export const contextSources = {
  list: (projectId: string, page = 1) =>
    request<ApiResponse<ContextSource[]>>(`/projects/${projectId}/context-sources?page=${page}`),
  create: (projectId: string, input: ContextSourceInput) =>
    request<ApiResponse<ContextSource>>(`/projects/${projectId}/context-sources`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  get: (id: string) =>
    request<ApiResponse<ContextSource>>(`/context-sources/${id}`),
  update: (id: string, input: ContextSourceInput) =>
    request<ApiResponse<ContextSource>>(`/context-sources/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (id: string) =>
    request<ApiResponse<null>>(`/context-sources/${id}`, { method: "DELETE" }),
};

// --- Semantic Releases ---

export const semanticReleases = {
  listAll: (page = 1, perPage = 25) =>
    request<ApiResponse<SemanticRelease[]>>(`/semantic-releases?page=${page}&per_page=${perPage}`),
  list: (projectId: string, page = 1, perPage = 25) =>
    request<ApiResponse<SemanticRelease[]>>(`/projects/${projectId}/semantic-releases?page=${page}&per_page=${perPage}`),
  get: (id: string) =>
    request<ApiResponse<SemanticRelease>>(`/semantic-releases/${id}`),
  delete: (id: string) =>
    request<ApiResponse<null>>(`/semantic-releases/${id}`, { method: "DELETE" }),
  getSources: (id: string) =>
    request<ApiResponse<Release[]>>(`/semantic-releases/${id}/sources`),
};

// --- Agent ---

export const agent = {
  triggerRun: (projectId: string, version?: string) =>
    request<ApiResponse<AgentRun>>(`/projects/${projectId}/agent/run`, {
      method: "POST",
      body: JSON.stringify({ trigger: "test", version: version || undefined }),
    }),
  listRuns: (projectId: string, page = 1) =>
    request<ApiResponse<AgentRun[]>>(`/projects/${projectId}/agent/runs?page=${page}`),
  getRun: (id: string) =>
    request<ApiResponse<AgentRun>>(`/agent-runs/${id}`),
};

// --- Subscriptions ---

export const subscriptions = {
  list: (page = 1, perPage = 100) =>
    request<ApiResponse<Subscription[]>>(`/subscriptions?page=${page}&per_page=${perPage}`),
  get: (id: string) => request<ApiResponse<Subscription>>(`/subscriptions/${id}`),
  create: (input: SubscriptionInput) =>
    request<ApiResponse<Subscription>>("/subscriptions", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  batchCreate: (input: BatchSubscriptionInput) =>
    request<ApiResponse<Subscription[]>>("/subscriptions/batch", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: string, input: SubscriptionInput) =>
    request<ApiResponse<Subscription>>(`/subscriptions/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (id: string) =>
    request<ApiResponse<null>>(`/subscriptions/${id}`, { method: "DELETE" }),
  batchDelete: (input: BatchDeleteSubscriptionInput) =>
    request<ApiResponse<null>>("/subscriptions/batch", {
      method: "DELETE",
      body: JSON.stringify(input),
    }),
};

// --- Notification Channels ---

export const channels = {
  list: (page = 1, perPage = 100) =>
    request<ApiResponse<NotificationChannel[]>>(`/channels?page=${page}&per_page=${perPage}`),
  get: (id: string) => request<ApiResponse<NotificationChannel>>(`/channels/${id}`),
  create: (input: ChannelInput) =>
    request<ApiResponse<NotificationChannel>>("/channels", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: string, input: ChannelInput) =>
    request<ApiResponse<NotificationChannel>>(`/channels/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (id: string) =>
    request<ApiResponse<null>>(`/channels/${id}`, { method: "DELETE" }),
  test: (id: string) =>
    request<ApiResponse<{ status: string }>>(`/channels/${id}/test`, {
      method: "POST",
    }),
};

// --- Todos ---

export interface TodoFilters {
  status?: string;
  project?: string;
  provider?: string;
  urgency?: string;
  date_from?: string;
  date_to?: string;
  aggregated?: boolean;
}

export const todos = {
  list: (page = 1, perPage = 25, filters?: TodoFilters) => {
    const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
    if (filters?.status) params.set("status", filters.status);
    if (filters?.aggregated) params.set("aggregated", "true");
    if (filters?.project) params.set("project", filters.project);
    if (filters?.provider) params.set("provider", filters.provider);
    if (filters?.urgency) params.set("urgency", filters.urgency);
    if (filters?.date_from) params.set("date_from", filters.date_from);
    if (filters?.date_to) params.set("date_to", filters.date_to);
    return request<ApiResponse<Todo[]>>(`/todos?${params}`);
  },
  get: (id: string) =>
    request<ApiResponse<Todo>>(`/todos/${id}`),
  acknowledge: (id: string, cascade = false) =>
    request<ApiResponse<{ status: string }>>(`/todos/${id}/acknowledge${cascade ? '?cascade=true' : ''}`, { method: "PATCH" }),
  resolve: (id: string, cascade = false) =>
    request<ApiResponse<{ status: string }>>(`/todos/${id}/resolve${cascade ? '?cascade=true' : ''}`, { method: "PATCH" }),
  reopen: (id: string) =>
    request<ApiResponse<{ status: string }>>(`/todos/${id}/reopen`, { method: "PATCH" }),
};

// --- System ---

export const system = {
  health: () => request<HealthStatus>("/health"),
  stats: () => request<ApiResponse<Stats>>("/stats"),
  trend: (granularity: "daily" | "weekly" | "monthly" = "daily", days = 7) =>
    request<ApiResponse<TrendData>>(`/stats/trend?granularity=${granularity}&days=${days}`),
};

// --- Discovery ---

export const discover = {
  github: (params?: { q?: string; language?: string }) => {
    const search = new URLSearchParams();
    if (params?.q) search.set("q", params.q);
    if (params?.language) search.set("language", params.language);
    const qs = search.toString();
    return request<ApiResponse<DiscoverItem[]>>(`/discover/github${qs ? `?${qs}` : ""}`);
  },
  dockerhub: (params?: { q?: string }) => {
    const search = new URLSearchParams();
    if (params?.q) search.set("q", params.q);
    const qs = search.toString();
    return request<ApiResponse<DiscoverItem[]>>(`/discover/dockerhub${qs ? `?${qs}` : ""}`);
  },
};

// --- Suggestions ---

export const suggestions = {
  stars: () =>
    request<ApiResponse<SuggestionItem[]>>("/suggestions/stars"),
  repos: () =>
    request<ApiResponse<RepoItem[]>>("/suggestions/repos"),
};

// --- Onboarding ---

export const onboard = {
  scan: async (repoUrl: string): Promise<ApiResponse<OnboardScan>> => {
    const res = await fetch(`${BASE}/onboard/scan`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ repo_url: repoUrl }),
    });
    // 409 returns the existing active scan — treat as success
    if (res.status === 409) {
      return res.json();
    }
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      throw new Error(body?.error?.message ?? `Request failed: ${res.status}`);
    }
    return res.json();
  },
  getScan: (id: string) =>
    request<ApiResponse<OnboardScan>>(`/onboard/scans/${id}`),
  apply: (id: string, selections: OnboardSelection[]) =>
    request<ApiResponse<OnboardApplyResult>>(`/onboard/scans/${id}/apply`, {
      method: "POST",
      body: JSON.stringify({ selections }),
    }),
};

// --- Release Gates ---

export const gates = {
  get: async (projectId: string): Promise<ApiResponse<ReleaseGate | null>> => {
    const res = await fetch(`${BASE}/projects/${projectId}/release-gate`, {
      headers: { "Content-Type": "application/json" },
    });
    if (res.status === 404) {
      return { data: null };
    }
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      throw new Error(body?.error?.message ?? `Request failed: ${res.status}`);
    }
    return res.json();
  },
  upsert: (projectId: string, input: ReleaseGateInput) =>
    request<ApiResponse<ReleaseGate>>(`/projects/${projectId}/release-gate`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  listReadiness: (projectId: string, page = 1, perPage = 25) =>
    request<ApiResponse<VersionReadiness[]>>(
      `/projects/${projectId}/version-readiness?page=${page}&per_page=${perPage}`
    ),
  getReadiness: (projectId: string, version: string) =>
    request<ApiResponse<VersionReadiness>>(
      `/projects/${projectId}/version-readiness/${encodeURIComponent(version)}`
    ),
  listEvents: (projectId: string, page = 1, perPage = 25) =>
    request<ApiResponse<GateEvent[]>>(
      `/projects/${projectId}/gate-events?page=${page}&per_page=${perPage}`
    ),
  listEventsByVersion: (projectId: string, version: string, page = 1, perPage = 25) =>
    request<ApiResponse<GateEvent[]>>(
      `/projects/${projectId}/version-readiness/${encodeURIComponent(version)}/events?page=${page}&per_page=${perPage}`
    ),
};

// --- API Keys ---

export const apiKeys = {
  list: (page = 1, perPage = 100) =>
    request<ApiResponse<ApiKey[]>>(`/api-keys?page=${page}&per_page=${perPage}`),
  create: (input: ApiKeyCreateInput) =>
    request<ApiResponse<ApiKey>>("/api-keys", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  delete: (id: string) =>
    request<ApiResponse<null>>(`/api-keys/${id}`, { method: "DELETE" }),
};
