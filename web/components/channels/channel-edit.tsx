"use client";

import useSWR from "swr";
import { useRouter } from "next/navigation";
import { channels as channelsApi } from "@/lib/api/client";
import { ChannelForm } from "@/components/channels/channel-form";
import { Button } from "@/components/ui/button";
import { Trash2 } from "lucide-react";

export function ChannelEdit({ id }: { id: string }) {
  const router = useRouter();
  const { data, isLoading } = useSWR(`channel-${id}`, () => channelsApi.get(id));

  const handleDelete = async () => {
    if (!confirm("Delete this channel?")) return;
    await channelsApi.delete(id);
    router.push("/channels");
  };

  if (isLoading) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;
  if (!data?.data) return <div className="py-12 text-center">Channel not found</div>;

  return (
    <div className="space-y-4">
      <ChannelForm
        title="Edit Channel"
        initial={data.data}
        onSubmit={async (input) => { await channelsApi.update(id, input); }}
      />
      <div className="mx-auto max-w-2xl flex justify-end">
        <Button variant="destructive" size="sm" onClick={handleDelete}>
          <Trash2 className="mr-2 h-4 w-4" /> Delete
        </Button>
      </div>
    </div>
  );
}
