"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslation } from "@/lib/i18n/context";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { NotificationChannel, ChannelInput } from "@/lib/api/types";

const channelFields: Record<string, { labelKey: string; placeholderKey: string; configKey: string }[]> = {
  slack: [
    { labelKey: "channelForm.slack.webhookUrl", placeholderKey: "channelForm.slack.webhookUrlPlaceholder", configKey: "webhook_url" },
    { labelKey: "channelForm.slack.channel", placeholderKey: "channelForm.slack.channelPlaceholder", configKey: "channel" },
  ],
  pagerduty: [
    { labelKey: "channelForm.pagerduty.routingKey", placeholderKey: "channelForm.pagerduty.routingKeyPlaceholder", configKey: "routing_key" },
  ],
  webhook: [
    { labelKey: "channelForm.webhook.url", placeholderKey: "channelForm.webhook.urlPlaceholder", configKey: "url" },
    { labelKey: "channelForm.webhook.headers", placeholderKey: "channelForm.webhook.headersPlaceholder", configKey: "headers" },
  ],
  email: [
    { labelKey: "channelForm.email.smtpHost", placeholderKey: "channelForm.email.smtpHostPlaceholder", configKey: "smtp_host" },
    { labelKey: "channelForm.email.smtpPort", placeholderKey: "channelForm.email.smtpPortPlaceholder", configKey: "smtp_port" },
    { labelKey: "channelForm.email.username", placeholderKey: "channelForm.email.usernamePlaceholder", configKey: "username" },
    { labelKey: "channelForm.email.password", placeholderKey: "channelForm.email.passwordPlaceholder", configKey: "password" },
    { labelKey: "channelForm.email.fromAddress", placeholderKey: "channelForm.email.fromAddressPlaceholder", configKey: "from_address" },
    { labelKey: "channelForm.email.toAddresses", placeholderKey: "channelForm.email.toAddressesPlaceholder", configKey: "to_addresses" },
  ],
};

interface ChannelFormProps {
  initial?: NotificationChannel;
  onSubmit: (input: ChannelInput) => Promise<void>;
  title: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}

export function ChannelForm({ initial, onSubmit, title, onSuccess, onCancel }: ChannelFormProps) {
  const router = useRouter();
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [type, setType] = useState(initial?.type ?? "slack");
  const [name, setName] = useState(initial?.name ?? "");
  const [config, setConfig] = useState<Record<string, unknown>>(initial?.config ?? {});

  const fields = channelFields[type] ?? [];

  const handleConfigChange = (configKey: string, value: string) => {
    setConfig((prev) => ({ ...prev, [configKey]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      let finalConfig = config;
      if (type === "email") {
        finalConfig = {
          ...config,
          smtp_port: Number(config.smtp_port) || 587,
          to_addresses: typeof config.to_addresses === "string"
            ? (config.to_addresses as string).split(",").map((s) => s.trim()).filter(Boolean)
            : config.to_addresses,
        };
      }
      await onSubmit({ type, name, config: finalConfig });
      if (onSuccess) {
        onSuccess();
      } else {
        router.push("/channels");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t("channelForm.failedToSave"));
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    } else {
      router.back();
    }
  };

  const formContent = (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}
      <div className="space-y-2">
        <Label htmlFor="name">{t("channelForm.name")}</Label>
        <Input id="name" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("channelForm.namePlaceholder")} required />
      </div>
      <div className="space-y-2">
        <Label>{t("channelForm.type")}</Label>
        <Select value={type} onValueChange={(v) => { setType(v); setConfig({}); }}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="slack">{t("channelForm.typeSlack")}</SelectItem>
            <SelectItem value="pagerduty">{t("channelForm.typePagerDuty")}</SelectItem>
            <SelectItem value="webhook">{t("channelForm.typeWebhook")}</SelectItem>
            <SelectItem value="email">{t("channelForm.typeEmail")}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {fields.map((field) => {
        return (
          <div key={field.configKey} className="space-y-2">
            <Label htmlFor={field.configKey}>{t(field.labelKey)}</Label>
            <Input id={field.configKey} value={String(config[field.configKey] ?? "")} onChange={(e) => handleConfigChange(field.configKey, e.target.value)} placeholder={t(field.placeholderKey)} />
          </div>
        );
      })}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={handleCancel}>{t("channelForm.cancel")}</Button>
        <Button type="submit" disabled={saving}>{saving ? t("channelForm.saving") : t("channelForm.save")}</Button>
      </div>
    </form>
  );

  if (onSuccess) {
    return formContent;
  }

  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader><CardTitle>{title}</CardTitle></CardHeader>
      <CardContent>
        {formContent}
      </CardContent>
    </Card>
  );
}
