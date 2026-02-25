"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { sources as sourcesApi } from "@/lib/api/client";
import { SourceForm } from "@/components/sources/source-form";

function NewSourceContent() {
  const searchParams = useSearchParams();
  const projectId = searchParams.get("project_id");

  if (!projectId) {
    return (
      <div className="py-12 text-center text-muted-foreground">
        Sources must be created within a project. Go to a project and add a source from there.
      </div>
    );
  }

  return (
    <SourceForm
      title="Add Source"
      projectId={projectId}
      onSubmit={async (input) => { await sourcesApi.create(projectId, input); }}
      redirectTo={`/projects/${projectId}`}
    />
  );
}

export default function NewSourcePage() {
  return (
    <Suspense fallback={<div className="py-12 text-center text-muted-foreground">Loading...</div>}>
      <NewSourceContent />
    </Suspense>
  );
}
