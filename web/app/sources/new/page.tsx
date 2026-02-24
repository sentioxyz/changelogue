"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { sources as sourcesApi } from "@/lib/api/client";
import { SourceForm } from "@/components/sources/source-form";

function NewSourceContent() {
  const searchParams = useSearchParams();
  const projectId = searchParams.get("project_id");
  return (
    <SourceForm
      title="Add Source"
      defaultProjectId={projectId ? Number(projectId) : undefined}
      onSubmit={async (input) => { await sourcesApi.create(input); }}
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
