"use client";

import useSWR from "swr";
import Link from "next/link";
import { projects as projectsApi } from "@/lib/api/client";
import { timeAgo } from "@/lib/format";
import { Plus } from "lucide-react";

export default function ProjectsPage() {
  const { data, isLoading } = useSWR("projects", () => projectsApi.list());
  const items = data?.data ?? [];

  return (
    <div className="flex flex-col gap-4 fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1
            className="text-[24px] font-semibold text-[#111113]"
            style={{ fontFamily: "var(--font-fraunces)" }}
          >
            Projects
          </h1>
          <p
            className="mt-1 text-[13px] text-[#6b7280]"
            style={{ fontFamily: "var(--font-dm-sans)" }}
          >
            Tracked software projects and their agent configurations.
          </p>
        </div>
        <Link
          href="/projects/new"
          className="flex items-center gap-1.5 rounded px-3 py-1.5 text-[13px] text-white transition-opacity hover:opacity-90"
          style={{ backgroundColor: "#e8601a", fontFamily: "var(--font-dm-sans)" }}
        >
          <Plus className="h-3.5 w-3.5" />
          New Project
        </Link>
      </div>

      <div
        className="overflow-hidden rounded-md"
        style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
      >
        {isLoading ? (
          <p
            className="px-4 py-8 text-center text-[14px] italic text-[#9ca3af]"
            style={{ fontFamily: "var(--font-fraunces)" }}
          >
            Loading…
          </p>
        ) : items.length === 0 ? (
          <p
            className="px-4 py-8 text-center text-[14px] italic text-[#9ca3af]"
            style={{ fontFamily: "var(--font-fraunces)" }}
          >
            No projects yet — create one to start tracking releases
          </p>
        ) : (
          <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans)" }}>
            <thead>
              <tr style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#fafaf9" }}>
                {["Name", "Description", "Agent Rules", "Created"].map((h) => (
                  <th
                    key={h}
                    className="px-4 py-2.5 text-left text-[11px] font-medium uppercase tracking-[0.08em] text-[#9ca3af]"
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items.map((project, i) => (
                <tr
                  key={project.id}
                  className="transition-colors duration-100 hover:bg-[#fafaf9]"
                  style={i > 0 ? { borderTop: "1px solid #e8e8e5" } : undefined}
                >
                  <td className="px-4 py-2.5">
                    <Link
                      href={`/projects/${project.id}`}
                      className="font-medium text-[#e8601a] hover:underline"
                    >
                      {project.name}
                    </Link>
                  </td>
                  <td className="max-w-[280px] truncate px-4 py-2.5 text-[#6b7280]">
                    {project.description || "—"}
                  </td>
                  <td className="px-4 py-2.5">
                    <div className="flex flex-wrap gap-1">
                      {project.agent_rules?.on_major_release && (
                        <span
                          className="rounded-full px-2 py-0.5 text-[11px] font-medium"
                          style={{ backgroundColor: "#f3f3f1", color: "#374151" }}
                        >
                          major
                        </span>
                      )}
                      {project.agent_rules?.on_minor_release && (
                        <span
                          className="rounded-full px-2 py-0.5 text-[11px] font-medium"
                          style={{ backgroundColor: "#f3f3f1", color: "#374151" }}
                        >
                          minor
                        </span>
                      )}
                      {project.agent_rules?.on_security_patch && (
                        <span
                          className="rounded-full px-2 py-0.5 text-[11px] font-medium"
                          style={{ backgroundColor: "#f3f3f1", color: "#374151" }}
                        >
                          security
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-2.5 text-[12px] text-[#9ca3af]">
                    {timeAgo(project.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
