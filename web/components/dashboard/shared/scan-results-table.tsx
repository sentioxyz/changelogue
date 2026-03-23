"use client";

import type { ScannedDependency, Project } from "@/lib/api/types";
import { ecosystemColors } from "./ecosystem-colors";

interface ScanResultsTableProps {
  deps: ScannedDependency[];
  selections: Record<number, boolean>;
  onSelectionsChange: (s: Record<number, boolean>) => void;
  projectAssignments: Record<number, { mode: "new" | "existing"; projectId?: string; newName?: string }>;
  onProjectAssignmentsChange: (a: Record<number, { mode: "new" | "existing"; projectId?: string; newName?: string }>) => void;
  existingProjects: Project[];
}

export function ScanResultsTable({
  deps,
  selections,
  onSelectionsChange,
  projectAssignments,
  onProjectAssignmentsChange,
  existingProjects,
}: ScanResultsTableProps) {
  const selectedCount = Object.values(selections).filter(Boolean).length;

  return (
    <div
      className="overflow-hidden rounded-md"
      style={{ border: "1px solid #e8e8e5", backgroundColor: "#ffffff" }}
    >
      <table className="w-full text-[13px]" style={{ fontFamily: "var(--font-dm-sans)" }}>
        <thead>
          <tr style={{ borderBottom: "1px solid #e8e8e5", backgroundColor: "#fafaf9" }}>
            <th className="w-10 px-3 py-2.5">
              <input
                type="checkbox"
                checked={selectedCount === deps.length}
                onChange={(e) => {
                  const val = e.target.checked;
                  const s: Record<number, boolean> = {};
                  deps.forEach((_, i) => { s[i] = val; });
                  onSelectionsChange(s);
                }}
                className="rounded"
                style={{ accentColor: "#e8601a" }}
              />
            </th>
            <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
              Dependency
            </th>
            <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
              Version
            </th>
            <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
              Ecosystem
            </th>
            <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
              Source
            </th>
            <th className="px-3 py-2.5 text-left text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: "#9ca3af" }}>
              Project
            </th>
          </tr>
        </thead>
        <tbody>
          {deps.map((dep, i) => {
            const eco = ecosystemColors[dep.ecosystem] || ecosystemColors.other;
            return (
              <tr
                key={i}
                className="transition-colors hover:bg-[#fafaf9]"
                style={{
                  borderBottom: i < deps.length - 1 ? "1px solid #e8e8e5" : undefined,
                  opacity: selections[i] ? 1 : 0.5,
                }}
              >
                <td className="px-3 py-2.5">
                  <input
                    type="checkbox"
                    checked={!!selections[i]}
                    onChange={(e) => onSelectionsChange({ ...selections, [i]: e.target.checked })}
                    className="rounded"
                    style={{ accentColor: "#e8601a" }}
                  />
                </td>
                <td className="px-3 py-2.5">
                  <span
                    className="text-[12px]"
                    style={{ fontFamily: "'JetBrains Mono', monospace", color: "#111113" }}
                  >
                    {dep.name}
                  </span>
                </td>
                <td className="px-3 py-2.5">
                  <span
                    className="inline-flex items-center rounded px-1.5 py-0.5 text-[11px]"
                    style={{ backgroundColor: "#f3f3f1", fontFamily: "'JetBrains Mono', monospace", color: "#374151" }}
                  >
                    {dep.version}
                  </span>
                </td>
                <td className="px-3 py-2.5">
                  <span
                    className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium"
                    style={{ backgroundColor: eco.bg, color: eco.text, border: `1px solid ${eco.border}` }}
                  >
                    {dep.ecosystem}
                  </span>
                </td>
                <td className="px-3 py-2.5">
                  <span className="text-[12px]" style={{ color: "#6b7280", fontFamily: "'JetBrains Mono', monospace" }}>
                    {dep.upstream_repo}
                  </span>
                </td>
                <td className="px-3 py-2.5">
                  <select
                    value={projectAssignments[i]?.mode === "existing" ? projectAssignments[i]?.projectId : "__new__"}
                    onChange={(e) => {
                      const val = e.target.value;
                      if (val === "__new__") {
                        onProjectAssignmentsChange({
                          ...projectAssignments,
                          [i]: { mode: "new", newName: dep.name.replace(/\//g, "-") },
                        });
                      } else {
                        onProjectAssignmentsChange({
                          ...projectAssignments,
                          [i]: { mode: "existing", projectId: val },
                        });
                      }
                    }}
                    className="rounded-md border px-2 py-1 text-[12px] bg-white"
                    style={{ borderColor: "#e8e8e5", color: "#374151", fontFamily: "var(--font-dm-sans)" }}
                  >
                    <option value="__new__">Create new project</option>
                    {existingProjects.map((p) => (
                      <option key={p.id} value={p.id}>{p.name}</option>
                    ))}
                  </select>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
