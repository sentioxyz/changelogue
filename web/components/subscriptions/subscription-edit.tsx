"use client";

import useSWR from "swr";
import { useRouter } from "next/navigation";
import { subscriptions as subsApi } from "@/lib/api/client";
import { SubscriptionForm } from "@/components/subscriptions/subscription-form";
import { Button } from "@/components/ui/button";
import { Trash2 } from "lucide-react";

export function SubscriptionEdit({ id }: { id: string }) {
  const router = useRouter();
  const { data, isLoading } = useSWR(`sub-${id}`, () => subsApi.get(id));

  const handleDelete = async () => {
    if (!confirm("Delete this subscription?")) return;
    await subsApi.delete(id);
    router.push("/subscriptions");
  };

  if (isLoading) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;
  if (!data?.data) return <div className="py-12 text-center">Subscription not found</div>;

  return (
    <div className="space-y-4">
      <SubscriptionForm
        title="Edit Subscription"
        initial={data.data}
        onSubmit={async (input) => { await subsApi.update(id, input); }}
      />
      <div className="mx-auto max-w-2xl flex justify-end">
        <Button variant="destructive" size="sm" onClick={handleDelete}>
          <Trash2 className="mr-2 h-4 w-4" /> Delete
        </Button>
      </div>
    </div>
  );
}
