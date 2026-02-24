// web/lib/mock/handlers.ts
import { http, HttpResponse, delay } from "msw";
import {
  mockProjects, mockReleases, mockPipelineStatuses,
  mockSources, mockSubscriptions, mockChannels,
  mockProviders, mockStats,
} from "./data";

const BASE = "/api/v1";
let nextProjectId = 100;
let nextSourceId = 100;
let nextSubId = 100;
let nextChannelId = 100;

function envelope<T>(data: T, meta: Record<string, unknown> = {}) {
  return { data, meta: { request_id: crypto.randomUUID(), ...meta } };
}

function listEnvelope<T>(data: T[], page = 1, perPage = 25) {
  const start = (page - 1) * perPage;
  const slice = data.slice(start, start + perPage);
  return envelope(slice, { page, per_page: perPage, total: data.length });
}

export const handlers = [
  // --- Projects ---
  http.get(`${BASE}/projects`, async ({ request }) => {
    await delay(100);
    const url = new URL(request.url);
    const page = Number(url.searchParams.get("page") ?? 1);
    const perPage = Number(url.searchParams.get("per_page") ?? 25);
    return HttpResponse.json(listEnvelope(mockProjects, page, perPage));
  }),

  http.post(`${BASE}/projects`, async ({ request }) => {
    await delay(150);
    const body = (await request.json()) as Record<string, unknown>;
    const project = {
      id: nextProjectId++,
      ...body,
      pipeline_config: body.pipeline_config ?? { changelog_summarizer: {}, urgency_scorer: {} },
      subscription_count: 0,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    mockProjects.push(project as any);
    return HttpResponse.json(envelope(project), { status: 201 });
  }),

  http.get(`${BASE}/projects/:id`, async ({ params }) => {
    await delay(80);
    const project = mockProjects.find((p) => p.id === Number(params.id));
    if (!project) return HttpResponse.json({ error: { code: "not_found", message: "Project not found" } }, { status: 404 });
    const projectWithSources = { ...project, sources: mockSources.filter((s) => s.project_id === project.id) };
    return HttpResponse.json(envelope(projectWithSources));
  }),

  http.put(`${BASE}/projects/:id`, async ({ params, request }) => {
    await delay(120);
    const idx = mockProjects.findIndex((p) => p.id === Number(params.id));
    if (idx === -1) return HttpResponse.json({ error: { code: "not_found", message: "Project not found" } }, { status: 404 });
    const body = (await request.json()) as Record<string, unknown>;
    mockProjects[idx] = { ...mockProjects[idx], ...body, updated_at: new Date().toISOString() };
    return HttpResponse.json(envelope(mockProjects[idx]));
  }),

  http.delete(`${BASE}/projects/:id`, async ({ params }) => {
    await delay(100);
    const idx = mockProjects.findIndex((p) => p.id === Number(params.id));
    if (idx === -1) return HttpResponse.json({ error: { code: "not_found", message: "Project not found" } }, { status: 404 });
    mockProjects.splice(idx, 1);
    return HttpResponse.json(envelope(null));
  }),

  // --- Releases ---
  http.get(`${BASE}/releases`, async ({ request }) => {
    await delay(100);
    const url = new URL(request.url);
    const page = Number(url.searchParams.get("page") ?? 1);
    const perPage = Number(url.searchParams.get("per_page") ?? 25);
    let filtered = [...mockReleases];
    const projectId = url.searchParams.get("project_id");
    if (projectId) filtered = filtered.filter((r) => r.project_id === Number(projectId));
    const sourceId = url.searchParams.get("source_id");
    if (sourceId) filtered = filtered.filter((r) => r.source_id === Number(sourceId));
    const preRelease = url.searchParams.get("pre_release");
    if (preRelease !== null) filtered = filtered.filter((r) => r.is_pre_release === (preRelease === "true"));
    const order = url.searchParams.get("order") ?? "desc";
    filtered.sort((a, b) => order === "desc" ? b.created_at.localeCompare(a.created_at) : a.created_at.localeCompare(b.created_at));
    return HttpResponse.json(listEnvelope(filtered, page, perPage));
  }),

  http.get(`${BASE}/releases/:id`, async ({ params }) => {
    await delay(80);
    const release = mockReleases.find((r) => r.id === params.id);
    if (!release) return HttpResponse.json({ error: { code: "not_found", message: "Release not found" } }, { status: 404 });
    return HttpResponse.json(envelope(release));
  }),

  http.get(`${BASE}/releases/:id/pipeline`, async ({ params }) => {
    await delay(80);
    const status = mockPipelineStatuses[params.id as string];
    if (!status) return HttpResponse.json({ error: { code: "not_found", message: "Pipeline not found" } }, { status: 404 });
    return HttpResponse.json(envelope(status));
  }),

  http.get(`${BASE}/releases/:id/notes`, async ({ params }) => {
    await delay(80);
    const release = mockReleases.find((r) => r.id === params.id);
    if (!release) return HttpResponse.json({ error: { code: "not_found", message: "Release not found" } }, { status: 404 });
    const notes = `## ${release.raw_version}\n\nChangelog for ${release.project_name} ${release.raw_version}.\n\n### Changes\n- Various bug fixes and improvements\n- Performance enhancements\n\n### Contributors\nThanks to all contributors!`;
    return HttpResponse.json(envelope(notes));
  }),

  // --- Sources ---
  http.get(`${BASE}/sources`, async () => {
    await delay(100);
    return HttpResponse.json(listEnvelope(mockSources));
  }),

  http.post(`${BASE}/sources`, async ({ request }) => {
    await delay(150);
    const body = (await request.json()) as Record<string, unknown>;
    const source = {
      id: nextSourceId++, ...body,
      last_polled_at: null, last_error: null,
      created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
    };
    mockSources.push(source as any);
    return HttpResponse.json(envelope(source), { status: 201 });
  }),

  http.get(`${BASE}/sources/:id`, async ({ params }) => {
    await delay(80);
    const source = mockSources.find((s) => s.id === Number(params.id));
    if (!source) return HttpResponse.json({ error: { code: "not_found", message: "Source not found" } }, { status: 404 });
    return HttpResponse.json(envelope(source));
  }),

  http.put(`${BASE}/sources/:id`, async ({ params, request }) => {
    await delay(120);
    const idx = mockSources.findIndex((s) => s.id === Number(params.id));
    if (idx === -1) return HttpResponse.json({ error: { code: "not_found", message: "Source not found" } }, { status: 404 });
    const body = (await request.json()) as Record<string, unknown>;
    mockSources[idx] = { ...mockSources[idx], ...body, updated_at: new Date().toISOString() };
    return HttpResponse.json(envelope(mockSources[idx]));
  }),

  http.delete(`${BASE}/sources/:id`, async ({ params }) => {
    await delay(100);
    const idx = mockSources.findIndex((s) => s.id === Number(params.id));
    if (idx === -1) return HttpResponse.json({ error: { code: "not_found", message: "Source not found" } }, { status: 404 });
    mockSources.splice(idx, 1);
    return HttpResponse.json(envelope(null));
  }),

  http.get(`${BASE}/sources/:id/latest-release`, async ({ params }) => {
    await delay(80);
    const rel = mockReleases.filter((r) => r.source_id === Number(params.id)).sort((a, b) => b.created_at.localeCompare(a.created_at))[0];
    if (!rel) return HttpResponse.json({ error: { code: "not_found", message: "No releases" } }, { status: 404 });
    return HttpResponse.json(envelope(rel));
  }),

  // --- Subscriptions ---
  http.get(`${BASE}/subscriptions`, async () => {
    await delay(100);
    return HttpResponse.json(listEnvelope(mockSubscriptions));
  }),

  http.post(`${BASE}/subscriptions`, async ({ request }) => {
    await delay(150);
    const body = (await request.json()) as Record<string, unknown>;
    const sub = {
      id: nextSubId++, ...body,
      enabled: body.enabled ?? true, frequency: body.frequency ?? "instant",
      created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
    };
    mockSubscriptions.push(sub as any);
    return HttpResponse.json(envelope(sub), { status: 201 });
  }),

  http.get(`${BASE}/subscriptions/:id`, async ({ params }) => {
    await delay(80);
    const sub = mockSubscriptions.find((s) => s.id === Number(params.id));
    if (!sub) return HttpResponse.json({ error: { code: "not_found", message: "Subscription not found" } }, { status: 404 });
    return HttpResponse.json(envelope(sub));
  }),

  http.put(`${BASE}/subscriptions/:id`, async ({ params, request }) => {
    await delay(120);
    const idx = mockSubscriptions.findIndex((s) => s.id === Number(params.id));
    if (idx === -1) return HttpResponse.json({ error: { code: "not_found", message: "Subscription not found" } }, { status: 404 });
    const body = (await request.json()) as Record<string, unknown>;
    mockSubscriptions[idx] = { ...mockSubscriptions[idx], ...body, updated_at: new Date().toISOString() };
    return HttpResponse.json(envelope(mockSubscriptions[idx]));
  }),

  http.delete(`${BASE}/subscriptions/:id`, async ({ params }) => {
    await delay(100);
    const idx = mockSubscriptions.findIndex((s) => s.id === Number(params.id));
    if (idx === -1) return HttpResponse.json({ error: { code: "not_found", message: "Subscription not found" } }, { status: 404 });
    mockSubscriptions.splice(idx, 1);
    return HttpResponse.json(envelope(null));
  }),

  // --- Channels ---
  http.get(`${BASE}/channels`, async () => {
    await delay(100);
    return HttpResponse.json(listEnvelope(mockChannels));
  }),

  http.post(`${BASE}/channels`, async ({ request }) => {
    await delay(150);
    const body = (await request.json()) as Record<string, unknown>;
    const channel = {
      id: nextChannelId++, ...body,
      enabled: body.enabled ?? true,
      created_at: new Date().toISOString(),
    };
    mockChannels.push(channel as any);
    return HttpResponse.json(envelope(channel), { status: 201 });
  }),

  http.get(`${BASE}/channels/:id`, async ({ params }) => {
    await delay(80);
    const ch = mockChannels.find((c) => c.id === Number(params.id));
    if (!ch) return HttpResponse.json({ error: { code: "not_found", message: "Channel not found" } }, { status: 404 });
    return HttpResponse.json(envelope(ch));
  }),

  http.put(`${BASE}/channels/:id`, async ({ params, request }) => {
    await delay(120);
    const idx = mockChannels.findIndex((c) => c.id === Number(params.id));
    if (idx === -1) return HttpResponse.json({ error: { code: "not_found", message: "Channel not found" } }, { status: 404 });
    const body = (await request.json()) as Record<string, unknown>;
    mockChannels[idx] = { ...mockChannels[idx], ...body };
    return HttpResponse.json(envelope(mockChannels[idx]));
  }),

  http.delete(`${BASE}/channels/:id`, async ({ params }) => {
    await delay(100);
    const idx = mockChannels.findIndex((c) => c.id === Number(params.id));
    if (idx === -1) return HttpResponse.json({ error: { code: "not_found", message: "Channel not found" } }, { status: 404 });
    mockChannels.splice(idx, 1);
    return HttpResponse.json(envelope(null));
  }),

  // --- Providers ---
  http.get(`${BASE}/providers`, async () => {
    await delay(50);
    return HttpResponse.json(envelope(mockProviders));
  }),

  // --- System ---
  http.get(`${BASE}/health`, async () => {
    return HttpResponse.json({ status: "healthy", checks: { database: "ok", queue: "ok" } });
  }),

  http.get(`${BASE}/stats`, async () => {
    await delay(80);
    return HttpResponse.json(envelope(mockStats));
  }),
];
