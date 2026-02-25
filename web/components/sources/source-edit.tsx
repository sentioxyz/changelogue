"use client";

import useSWR from "swr";
import { useRouter } from "next/navigation";
import { sources as sourcesApi } from "@/lib/api/client";
import { SourceForm } from "@/components/sources/source-form";
import { Button } from "@/components/ui/button";
import { Trash2 } from "lucide-react";

export function SourceEdit({ id }: { id: string }) {
  const router = useRouter();
  const { data, isLoading } = useSWR(`source-${id}`, () => sourcesApi.get(id));

  const handleDelete = async () => {
    if (!confirm("Delete this source?")) return;
    await sourcesApi.delete(id);
    router.push("/sources");
  };

  if (isLoading) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;
  if (!data?.data) return <div className="py-12 text-center">Source not found</div>;

  const source = data.data;

  return (
    <div className="space-y-4">
      <SourceForm
        title="Edit Source"
        initial={source}
        projectId={source.project_id}
        onSubmit={async (input) => { await sourcesApi.update(id, input); }}
        redirectTo={`/projects/${source.project_id}`}
      />
      <div className="mx-auto max-w-2xl flex justify-end">
        <Button variant="destructive" size="sm" onClick={handleDelete}>
          <Trash2 className="mr-2 h-4 w-4" /> Delete Source
        </Button>
      </div>
    </div>
  );
}
