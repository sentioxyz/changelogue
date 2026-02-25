import { NewContextSourceForm } from "@/components/context-sources/new-context-source-form";

export async function generateStaticParams() {
  return [{ id: "0" }];
}

export default async function NewContextSourcePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <NewContextSourceForm projectId={id} />;
}
