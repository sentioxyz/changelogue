// web/lib/api/client.ts
import type {
  ApiResponse,
  Project,
  ProjectInput,
  Release,
  PipelineStatus,
  Source,
  SourceInput,
  Subscription,
  SubscriptionInput,
  NotificationChannel,
  ChannelInput,
  Provider,
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
  get: (id: number) =>
    request<ApiResponse<Project>>(`/projects/${id}`),
  create: (input: ProjectInput) =>
    request<ApiResponse<Project>>("/projects", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: number, input: ProjectInput) =>
    request<ApiResponse<Project>>(`/projects/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (id: number) =>
    request<ApiResponse<null>>(`/projects/${id}`, { method: "DELETE" }),
};

// --- Releases ---

export interface ListReleasesParams {
  project_id?: number;
  source_id?: number;
  pre_release?: boolean;
  page?: number;
  per_page?: number;
  sort?: string;
  order?: "asc" | "desc";
}

export const releases = {
  list: (params: ListReleasesParams = {}) => {
    const qs = new URLSearchParams();
    if (params.project_id) qs.set("project_id", String(params.project_id));
    if (params.source_id) qs.set("source_id", String(params.source_id));
    if (params.pre_release !== undefined) qs.set("pre_release", String(params.pre_release));
    qs.set("page", String(params.page ?? 1));
    qs.set("per_page", String(params.per_page ?? 25));
    if (params.sort) qs.set("sort", params.sort);
    if (params.order) qs.set("order", params.order);
    return request<ApiResponse<Release[]>>(`/releases?${qs}`);
  },
  get: (id: string) =>
    request<ApiResponse<Release>>(`/releases/${id}`),
  pipeline: (id: string) =>
    request<ApiResponse<PipelineStatus>>(`/releases/${id}/pipeline`),
  notes: (id: string) =>
    request<ApiResponse<string>>(`/releases/${id}/notes`),
};

// --- Sources ---

export const sources = {
  list: () => request<ApiResponse<Source[]>>("/sources"),
  get: (id: number) => request<ApiResponse<Source>>(`/sources/${id}`),
  create: (input: SourceInput) =>
    request<ApiResponse<Source>>("/sources", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: number, input: SourceInput) =>
    request<ApiResponse<Source>>(`/sources/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (id: number) =>
    request<ApiResponse<null>>(`/sources/${id}`, { method: "DELETE" }),
  latestRelease: (id: number) =>
    request<ApiResponse<Release>>(`/sources/${id}/latest-release`),
};

// --- Subscriptions ---

export const subscriptions = {
  list: () => request<ApiResponse<Subscription[]>>("/subscriptions"),
  get: (id: number) => request<ApiResponse<Subscription>>(`/subscriptions/${id}`),
  create: (input: SubscriptionInput) =>
    request<ApiResponse<Subscription>>("/subscriptions", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: number, input: SubscriptionInput) =>
    request<ApiResponse<Subscription>>(`/subscriptions/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (id: number) =>
    request<ApiResponse<null>>(`/subscriptions/${id}`, { method: "DELETE" }),
};

// --- Notification Channels ---

export const channels = {
  list: () => request<ApiResponse<NotificationChannel[]>>("/channels"),
  get: (id: number) => request<ApiResponse<NotificationChannel>>(`/channels/${id}`),
  create: (input: ChannelInput) =>
    request<ApiResponse<NotificationChannel>>("/channels", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: number, input: ChannelInput) =>
    request<ApiResponse<NotificationChannel>>(`/channels/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  delete: (id: number) =>
    request<ApiResponse<null>>(`/channels/${id}`, { method: "DELETE" }),
};

// --- System ---

export const system = {
  health: () => request<HealthStatus>("/health"),
  stats: () => request<ApiResponse<Stats>>("/stats"),
  providers: () => request<ApiResponse<Provider[]>>("/providers"),
};
