import { SourceEdit } from "@/components/sources/source-edit";

export function generateStaticParams() {
  return [{ id: "0" }];
}

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <SourceEdit id={id} />;
}
