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
  Settings,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/lib/auth/context";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SettingsDialog } from "@/components/settings/settings-dialog";
import { useTranslation } from "@/lib/i18n/context";

const navKeys = [
  { href: "/", key: "nav.dashboard", icon: LayoutDashboard },
  { href: "/projects", key: "nav.projects", icon: FolderKanban },
  { href: "/todo", key: "nav.todo", icon: ListTodo },
  { href: "/releases", key: "nav.releases", icon: Package },
  { href: "/channels", key: "nav.channels", icon: Megaphone },
  { href: "/subscriptions", key: "nav.subscriptions", icon: Bell },
];

export function Sidebar() {
  const pathname = usePathname();
  const [expanded, setExpanded] = useState(false);
  const { user, logout } = useAuth();
  const [settingsOpen, setSettingsOpen] = useState(false);
  const { t } = useTranslation();

  return (
    <aside
      className={cn(
        "flex shrink-0 flex-col transition-[width] duration-200",
        expanded ? "w-[200px]" : "w-[52px]"
      )}
      style={{ backgroundColor: "var(--sidebar-bg)" }}
    >
      {/* Logo + toggle */}
      <div className="flex h-12 items-center px-3">
        {expanded ? (
          <>
            <img src="/logo.svg" alt="" className="h-7 w-7 shrink-0" />
            <Link
              href="/"
              className="ml-2 text-[16px] font-semibold text-white"
              style={{ fontFamily: "var(--font-raleway)" }}
            >
              Changelogue
            </Link>
            <button
              onClick={() => setExpanded(false)}
              className="ml-auto text-sidebar-text transition-colors hover:text-white"
              title="Collapse sidebar"
            >
              <PanelLeftClose className="h-4 w-4" />
            </button>
          </>
        ) : (
          <button
            onClick={() => setExpanded(true)}
            className="mx-auto text-sidebar-text transition-colors hover:text-white"
            title="Expand sidebar"
          >
            <PanelLeftOpen className="h-4 w-4" />
          </button>
        )}
      </div>

      {/* Nav */}
      <nav className="flex-1 pt-2">
        {navKeys.map((item) => {
          const isActive =
            item.href === "/"
              ? pathname === "/"
              : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              title={expanded ? undefined : t(item.key)}
              className={cn(
                "flex items-center gap-3 py-2 text-[13px] transition-colors duration-150",
                expanded ? "pl-4 pr-3" : "justify-center px-0",
                isActive
                  ? "text-white"
                  : "text-sidebar-text hover:text-white"
              )}
              style={
                isActive
                  ? {
                      borderLeft: "3px solid var(--beacon-accent)",
                      backgroundColor: "rgba(255,255,255,0.06)",
                      paddingLeft: expanded ? "13px" : undefined,
                    }
                  : { borderLeft: "3px solid transparent" }
              }
            >
              <item.icon className="h-4 w-4 shrink-0" />
              {expanded && (
                <span style={{ fontFamily: "var(--font-dm-sans)" }}>
                  {t(item.key)}
                </span>
              )}
            </Link>
          );
        })}
      </nav>

      {/* User section */}
      {user && (
        <div className="border-t border-[rgba(255,255,255,0.1)] p-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                className={cn(
                  "flex w-full items-center gap-2 rounded-md p-1 transition-colors hover:bg-[rgba(255,255,255,0.06)]",
                  expanded ? "px-2" : "justify-center"
                )}
              >
                {user.avatar_url ? (
                  <img
                    src={user.avatar_url}
                    alt={user.github_login}
                    className="h-6 w-6 shrink-0 rounded-full"
                  />
                ) : (
                  <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[var(--beacon-accent)] text-xs text-white">
                    {user.github_login[0].toUpperCase()}
                  </div>
                )}
                {expanded && (
                  <span className="truncate text-xs text-sidebar-text">
                    {user.github_login}
                  </span>
                )}
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent side="right" align="end" className="w-48">
              <DropdownMenuItem onClick={() => setSettingsOpen(true)}>
                <Settings className="mr-2 h-4 w-4" />
                {t("user.settings")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={logout}>
                <LogOut className="mr-2 h-4 w-4" />
                {t("user.signout")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
        </div>
      )}
    </aside>
  );
}
