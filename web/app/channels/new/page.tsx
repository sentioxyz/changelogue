"use client";

import { channels as channelsApi } from "@/lib/api/client";
import { ChannelForm } from "@/components/channels/channel-form";

export default function NewChannelPage() {
  return (
    <ChannelForm
      title="Add Channel"
      onSubmit={async (input) => { await channelsApi.create(input); }}
    />
  );
}
