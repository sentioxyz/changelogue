import { FaSlack, FaDiscord } from "react-icons/fa";
import { TbWebhook } from "react-icons/tb";
import { HiOutlineMail } from "react-icons/hi";
import type { IconType } from "react-icons";

const TYPE_STYLES: Record<string, { bg: string; text: string; icon: IconType }> = {
  slack: { bg: "#4A154B", text: "#ffffff", icon: FaSlack },
  discord: { bg: "#5865F2", text: "#ffffff", icon: FaDiscord },
  webhook: { bg: "#1a1a1a", text: "#ffffff", icon: TbWebhook },
  email: { bg: "#2563EB", text: "#ffffff", icon: HiOutlineMail },
};

export function ChannelTypeBadge({ type }: { type: string }) {
  const style = TYPE_STYLES[type.toLowerCase()];
  const colors = style ?? { bg: "#6b7280", text: "#ffffff" };
  const Icon = style?.icon;
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5"
      style={{
        backgroundColor: colors.bg,
        color: colors.text,
        fontFamily: "var(--font-dm-sans)",
        fontSize: "12px",
        fontWeight: 500,
        lineHeight: "16px",
      }}
    >
      {Icon && <Icon size={12} />}
      {type}
    </span>
  );
}
