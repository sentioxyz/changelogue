"use client";

import { subscriptions as subsApi } from "@/lib/api/client";
import { SubscriptionForm } from "@/components/subscriptions/subscription-form";

export default function NewSubscriptionPage() {
  return (
    <SubscriptionForm
      title="Create Subscription"
      onSubmit={async (input) => { await subsApi.create(input); }}
    />
  );
}
