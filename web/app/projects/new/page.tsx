"use client";

import { projects as projectsApi } from "@/lib/api/client";
import { ProjectForm } from "@/components/projects/project-form";

export default function NewProjectPage() {
  return (
    <ProjectForm
      title="Create Project"
      onSubmit={async (input) => { await projectsApi.create(input); }}
    />
  );
}
