import { ProjectEdit } from "@/components/projects/project-edit";

// Return a placeholder param to work around Next.js static export requiring
// at least one entry from generateStaticParams (see vercel/next.js#71862).
// Real project pages are rendered client-side via SWR after navigation.
export async function generateStaticParams() {
  return [{ id: "0" }];
}

export default async function EditProjectPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ProjectEdit id={id} />;
}
