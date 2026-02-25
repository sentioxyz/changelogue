import { SemanticReleasesList } from "@/components/semantic-releases/semantic-releases-list";

export async function generateStaticParams() {
  return [{ id: "0" }];
}

export default async function SemanticReleasesPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <SemanticReleasesList projectId={id} />;
}
