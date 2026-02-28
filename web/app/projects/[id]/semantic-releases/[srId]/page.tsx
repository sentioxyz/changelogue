import { SemanticReleaseDetail } from "@/components/semantic-releases/semantic-release-detail";

export async function generateStaticParams() {
  return [{ id: "0", srId: "0" }];
}

export default function SemanticReleaseDetailPage() {
  return <SemanticReleaseDetail />;
}
