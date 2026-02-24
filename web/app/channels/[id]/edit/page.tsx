import { ChannelEdit } from "@/components/channels/channel-edit";

export function generateStaticParams() {
  return [{ id: "0" }];
}

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ChannelEdit id={id} />;
}
