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
  NotificationChannel,
  ChannelInput,
  ContextSource,
  ContextSourceInput,
  SemanticRelease,
  AgentRun,
  HealthStatus,
  Stats,
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
};

// --- Releases ---

export const releases = {
  listBySource: (sourceId: string, page = 1) =>
    request<ApiResponse<Release[]>>(`/sources/${sourceId}/releases?page=${page}`),
  listByProject: (projectId: string, page = 1) =>
    request<ApiResponse<Release[]>>(`/projects/${projectId}/releases?page=${page}`),
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
  list: (projectId: string, page = 1) =>
    request<ApiResponse<SemanticRelease[]>>(`/projects/${projectId}/semantic-releases?page=${page}`),
  get: (id: string) =>
    request<ApiResponse<SemanticRelease>>(`/semantic-releases/${id}`),
};

// --- Agent ---

export const agent = {
  triggerRun: (projectId: string) =>
    request<ApiResponse<AgentRun>>(`/projects/${projectId}/agent/run`, { method: "POST" }),
  listRuns: (projectId: string, page = 1) =>
    request<ApiResponse<AgentRun[]>>(`/projects/${projectId}/agent/runs?page=${page}`),
  getRun: (id: string) =>
    request<ApiResponse<AgentRun>>(`/agent-runs/${id}`),
};

// --- Subscriptions ---

export const subscriptions = {
  list: () => request<ApiResponse<Subscription[]>>("/subscriptions"),
  get: (id: string) => request<ApiResponse<Subscription>>(`/subscriptions/${id}`),
  create: (input: SubscriptionInput) =>
    request<ApiResponse<Subscription>>("/subscriptions", {
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
};

// --- Notification Channels ---

export const channels = {
  list: () => request<ApiResponse<NotificationChannel[]>>("/channels"),
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
};

// --- System ---

export const system = {
  health: () => request<HealthStatus>("/health"),
  stats: () => request<ApiResponse<Stats>>("/stats"),
};
