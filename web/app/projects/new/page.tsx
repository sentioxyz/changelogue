"use client";

import { projects as projectsApi, sources as sourcesApi } from "@/lib/api/client";
import { ProjectForm } from "@/components/projects/project-form";

export default function NewProjectPage() {
  return (
    <ProjectForm
      title="Create Project"
      onSubmit={async (result) => {
        const resp = await projectsApi.create(result.project);
        if (result.source && resp.data?.id) {
          await sourcesApi.create(resp.data.id, result.source);
        }
      }}
    />
  );
}
