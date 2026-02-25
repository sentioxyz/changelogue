"use client";

import { sources as sourcesApi } from "@/lib/api/client";
import { SourceForm } from "@/components/sources/source-form";

export function NewProjectSource({ projectId }: { projectId: string }) {
  return (
    <SourceForm
      title="Add Source"
      projectId={projectId}
      onSubmit={async (input) => { await sourcesApi.create(projectId, input); }}
      redirectTo={`/projects/${projectId}`}
    />
  );
}
