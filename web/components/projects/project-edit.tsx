"use client";

import useSWR from "swr";
import { projects as projectsApi } from "@/lib/api/client";
import { ProjectForm } from "@/components/projects/project-form";

export function ProjectEdit({ id }: { id: string }) {
  const { data, isLoading } = useSWR(`project-${id}`, () => projectsApi.get(id));

  if (isLoading) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;
  if (!data?.data) return <div className="py-12 text-center">Project not found</div>;

  return (
    <ProjectForm
      title="Edit Project"
      initial={data.data}
      onSubmit={async (input) => { await projectsApi.update(id, input); }}
    />
  );
}
