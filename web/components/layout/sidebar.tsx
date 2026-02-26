"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  FolderKanban,
  Package,
  Bell,
  Megaphone,
} from "lucide-react";
import { cn } from "@/lib/utils";

const navItems = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/releases", label: "Releases", icon: Package },
  { href: "/channels", label: "Channels", icon: Megaphone },
  { href: "/subscriptions", label: "Subscriptions", icon: Bell },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside
      className="flex w-[200px] shrink-0 flex-col"
      style={{ backgroundColor: "#16181c" }}
    >
      {/* Logo */}
      <div className="flex h-12 items-center gap-2 px-4">
        <img src="/logo.svg" alt="" className="h-7 w-7 shrink-0" />
        <Link
          href="/"
          className="text-[16px] italic text-white"
          style={{ fontFamily: "var(--font-fraunces)" }}
        >
          Changelogue
        </Link>
      </div>

      {/* Nav */}
      <nav className="flex-1 px-0 pt-2">
        {navItems.map((item) => {
          const isActive =
            item.href === "/"
              ? pathname === "/"
              : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 py-2 pl-4 pr-3 text-[13px] transition-colors duration-150",
                isActive
                  ? "text-white"
                  : "text-[#9ca3af] hover:text-white"
              )}
              style={
                isActive
                  ? {
                      borderLeft: "3px solid #e8601a",
                      backgroundColor: "rgba(255,255,255,0.06)",
                      paddingLeft: "13px",
                    }
                  : { borderLeft: "3px solid transparent" }
              }
            >
              <item.icon className="h-4 w-4 shrink-0" />
              <span style={{ fontFamily: "var(--font-dm-sans)" }}>{item.label}</span>
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
