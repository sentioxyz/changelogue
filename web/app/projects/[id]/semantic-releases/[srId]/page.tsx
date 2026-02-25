import { SemanticReleaseDetail } from "@/components/semantic-releases/semantic-release-detail";

export async function generateStaticParams() {
  return [{ id: "0", srId: "0" }];
}

export default async function SemanticReleaseDetailPage({
  params,
}: {
  params: Promise<{ id: string; srId: string }>;
}) {
  const { id, srId } = await params;
  return <SemanticReleaseDetail projectId={id} srId={srId} />;
}
