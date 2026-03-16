"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  ListTodo,
  FolderKanban,
  Package,
  Bell,
  Megaphone,
  PanelLeftOpen,
  PanelLeftClose,
  LogOut,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/lib/auth/context";

const navItems = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/todo", label: "Todo", icon: ListTodo },
  { href: "/releases", label: "Releases", icon: Package },
  { href: "/channels", label: "Channels", icon: Megaphone },
  { href: "/subscriptions", label: "Subscriptions", icon: Bell },
];

export function Sidebar() {
  const pathname = usePathname();
  const [expanded, setExpanded] = useState(false);
  const { user, logout } = useAuth();

  return (
    <aside
      className={cn(
        "flex shrink-0 flex-col transition-[width] duration-200",
        expanded ? "w-[200px]" : "w-[52px]"
      )}
      style={{ backgroundColor: "#16181c" }}
    >
      {/* Logo + toggle */}
      <div className="flex h-12 items-center px-3">
        {expanded ? (
          <>
            <img src="/logo.svg" alt="" className="h-7 w-7 shrink-0" />
            <Link
              href="/"
              className="ml-2 text-[16px] italic text-white"
              style={{ fontFamily: "var(--font-fraunces)" }}
            >
              Changelogue
            </Link>
            <button
              onClick={() => setExpanded(false)}
              className="ml-auto text-[#9ca3af] transition-colors hover:text-white"
              title="Collapse sidebar"
            >
              <PanelLeftClose className="h-4 w-4" />
            </button>
          </>
        ) : (
          <button
            onClick={() => setExpanded(true)}
            className="mx-auto text-[#9ca3af] transition-colors hover:text-white"
            title="Expand sidebar"
          >
            <PanelLeftOpen className="h-4 w-4" />
          </button>
        )}
      </div>

      {/* Nav */}
      <nav className="flex-1 pt-2">
        {navItems.map((item) => {
          const isActive =
            item.href === "/"
              ? pathname === "/"
              : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              title={expanded ? undefined : item.label}
              className={cn(
                "flex items-center gap-3 py-2 text-[13px] transition-colors duration-150",
                expanded ? "pl-4 pr-3" : "justify-center px-0",
                isActive
                  ? "text-white"
                  : "text-[#9ca3af] hover:text-white"
              )}
              style={
                isActive
                  ? {
                      borderLeft: "3px solid #e8601a",
                      backgroundColor: "rgba(255,255,255,0.06)",
                      paddingLeft: expanded ? "13px" : undefined,
                    }
                  : { borderLeft: "3px solid transparent" }
              }
            >
              <item.icon className="h-4 w-4 shrink-0" />
              {expanded && (
                <span style={{ fontFamily: "var(--font-dm-sans)" }}>
                  {item.label}
                </span>
              )}
            </Link>
          );
        })}
      </nav>

      {/* User section */}
      {user && (
        <div className="border-t border-[rgba(255,255,255,0.1)] p-2">
          <div className={cn("flex items-center gap-2", expanded ? "px-2" : "justify-center")}>
            {user.avatar_url ? (
              <img
                src={user.avatar_url}
                alt={user.github_login}
                className="h-6 w-6 shrink-0 rounded-full"
              />
            ) : (
              <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[#e8601a] text-xs text-white">
                {user.github_login[0].toUpperCase()}
              </div>
            )}
            {expanded && (
              <div className="flex flex-1 items-center justify-between">
                <span className="truncate text-xs text-[#9ca3af]">{user.github_login}</span>
                <button
                  onClick={logout}
                  title="Sign out"
                  className="text-[#9ca3af] transition-colors hover:text-white"
                >
                  <LogOut className="h-3.5 w-3.5" />
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </aside>
  );
}
