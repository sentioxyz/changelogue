import { NewProjectSource } from "@/components/sources/new-project-source";

export async function generateStaticParams() {
  return [{ id: "0" }];
}

export default async function NewProjectSourcePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <NewProjectSource projectId={id} />;
}
