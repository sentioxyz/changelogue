// web/components/layout/header.tsx
"use client";

import { usePathname } from "next/navigation";

const titles: Record<string, string> = {
  "/": "Dashboard",
  "/projects": "Projects",
  "/releases": "Releases",
  "/sources": "Sources",
  "/subscriptions": "Subscriptions",
  "/channels": "Channels",
};

export function Header() {
  const pathname = usePathname();
  const segment = "/" + (pathname.split("/")[1] ?? "");
  const title = titles[segment] ?? "ReleaseBeacon";

  return (
    <header className="flex h-14 items-center border-b px-6">
      <h1 className="text-lg font-semibold">{title}</h1>
    </header>
  );
}
