import { AgentPageContent } from "@/components/agent/agent-page";

export async function generateStaticParams() {
  return [{ id: "0" }];
}

export default function AgentPage() {
  return <AgentPageContent />;
}
