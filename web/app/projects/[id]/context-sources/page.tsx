import { ContextSourcesList } from "@/components/context-sources/context-sources-list";

export async function generateStaticParams() {
  return [{ id: "0" }];
}

export default async function ContextSourcesPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ContextSourcesList projectId={id} />;
}
