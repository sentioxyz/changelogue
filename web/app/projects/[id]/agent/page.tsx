import { AgentPageContent } from "@/components/agent/agent-page";

export async function generateStaticParams() {
  return [{ id: "0" }];
}

export default async function AgentPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <AgentPageContent projectId={id} />;
}
