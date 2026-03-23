# System Settings (Theme & Language) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a settings dialog accessible from the sidebar user menu that lets users toggle light/dark/system theme and switch between English and Chinese.

**Architecture:** `next-themes` ThemeProvider + custom LanguageProvider context wrap the app. Settings dialog uses existing shadcn Dialog/DropdownMenu/Select components. All preferences persist in localStorage.

**Tech Stack:** Next.js 16 (App Router), next-themes, React Context, Tailwind CSS v4, shadcn/ui, Lucide icons

**Spec:** `docs/superpowers/specs/2026-03-23-system-settings-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `web/lib/i18n/context.tsx` | Create | LanguageProvider + useTranslation hook |
| `web/lib/i18n/messages/en.json` | Create | English translation strings |
| `web/lib/i18n/messages/zh.json` | Create | Chinese translation strings |
| `web/components/providers.tsx` | Create | Client component wrapping ThemeProvider + LanguageProvider |
| `web/components/settings/settings-dialog.tsx` | Create | Settings modal UI |
| `web/app/globals.css` | Modify | Add `html.dark` CSS variables block |
| `web/app/layout.tsx` | Modify | Import Providers, add suppressHydrationWarning |
| `web/components/layout/sidebar.tsx` | Modify | Replace user section with DropdownMenu, refactor hardcoded colors |

---

### Task 1: Install next-themes

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install the package**

```bash
cd web && npm install next-themes
```

- [ ] **Step 2: Verify installation**

```bash
cd web && node -e "require('next-themes')" && echo "OK"
```
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add web/package.json web/package-lock.json
git commit -m "chore: add next-themes dependency"
```

---

### Task 2: Add dark mode CSS variables

**Files:**
- Modify: `web/app/globals.css`

- [ ] **Step 1: Add `html.dark` variable block after the `:root` block (after line 79)**

Insert this block right after the closing `}` of `:root`:

```css
html.dark {
  --background: #0f1115;
  --foreground: #e5e7eb;
  --surface: #16181c;
  --sidebar-bg: #0a0b0e;
  --sidebar-text: #6b7280;
  --beacon-accent: #e8601a;
  --border: #2a2d33;
  --mono-bg: #1e2025;
  --text-secondary: #9ca3af;
  --text-muted: #6b7280;

  /* shadcn variable mappings */
  --popover: #16181c;
  --popover-foreground: #e5e7eb;
  --card: #16181c;
  --card-foreground: #e5e7eb;
  --primary: #e8601a;
  --primary-foreground: #ffffff;
  --secondary: #1e2025;
  --secondary-foreground: #d1d5db;
  --muted: #1e2025;
  --muted-foreground: #9ca3af;
  --accent: #1e2025;
  --accent-foreground: #e5e7eb;
  --input: #2a2d33;
  --ring: #e8601a;
}
```

- [ ] **Step 2: Update the `--color-popover` and `--color-popover-foreground` hardcoded values in `@theme inline` to use variables**

In the `@theme inline` block, change:
```css
  --color-popover: #ffffff;
  --color-popover-foreground: #111113;
```
to:
```css
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
```

- [ ] **Step 3: Verify the CSS parses correctly**

```bash
cd web && npx next build 2>&1 | head -20
```
Expected: No CSS parse errors.

- [ ] **Step 4: Commit**

```bash
git add web/app/globals.css
git commit -m "feat: add dark mode CSS variables"
```

---

### Task 3: Create i18n context and message files

**Files:**
- Create: `web/lib/i18n/messages/en.json`
- Create: `web/lib/i18n/messages/zh.json`
- Create: `web/lib/i18n/context.tsx`

- [ ] **Step 1: Create English messages file**

Create `web/lib/i18n/messages/en.json`:

```json
{
  "nav.dashboard": "Dashboard",
  "nav.projects": "Projects",
  "nav.todo": "Todo",
  "nav.releases": "Releases",
  "nav.channels": "Channels",
  "nav.subscriptions": "Subscriptions",
  "settings.title": "Settings",
  "settings.theme": "Theme",
  "settings.theme.light": "Light",
  "settings.theme.dark": "Dark",
  "settings.theme.system": "System",
  "settings.language": "Language",
  "settings.language.en": "English",
  "settings.language.zh": "中文",
  "user.signout": "Sign out",
  "user.settings": "Settings"
}
```

- [ ] **Step 2: Create Chinese messages file**

Create `web/lib/i18n/messages/zh.json`:

```json
{
  "nav.dashboard": "仪表盘",
  "nav.projects": "项目",
  "nav.todo": "待办",
  "nav.releases": "发布",
  "nav.channels": "频道",
  "nav.subscriptions": "订阅",
  "settings.title": "设置",
  "settings.theme": "主题",
  "settings.theme.light": "浅色",
  "settings.theme.dark": "深色",
  "settings.theme.system": "跟随系统",
  "settings.language": "语言",
  "settings.language.en": "English",
  "settings.language.zh": "中文",
  "user.signout": "退出登录",
  "user.settings": "设置"
}
```

- [ ] **Step 3: Create the LanguageProvider context**

Create `web/lib/i18n/context.tsx`:

```tsx
"use client";

import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import en from "./messages/en.json";
import zh from "./messages/zh.json";

const messages: Record<string, Record<string, string>> = { en, zh };

type Locale = "en" | "zh";

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string) => string;
}

const I18nContext = createContext<I18nContextValue | null>(null);

const STORAGE_KEY = "changelogue-locale";

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>("en");

  // Read from localStorage on mount
  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "en" || stored === "zh") {
      setLocaleState(stored);
      document.documentElement.lang = stored;
    }
  }, []);

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    localStorage.setItem(STORAGE_KEY, next);
    document.documentElement.lang = next;
  }, []);

  const t = useCallback(
    (key: string): string => {
      return messages[locale]?.[key] ?? messages.en?.[key] ?? key;
    },
    [locale]
  );

  return (
    <I18nContext.Provider value={{ locale, setLocale, t }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useTranslation() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useTranslation must be used within LanguageProvider");
  return ctx;
}
```

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd web && npx tsc --noEmit 2>&1 | grep -i "i18n" || echo "No i18n errors"
```
Expected: `No i18n errors`

- [ ] **Step 5: Commit**

```bash
git add web/lib/i18n/
git commit -m "feat: add i18n context with EN/ZH translations"
```

---

### Task 4: Create Providers wrapper and wire into root layout

**Files:**
- Create: `web/components/providers.tsx`
- Modify: `web/app/layout.tsx`

- [ ] **Step 1: Create the client-side Providers wrapper**

`layout.tsx` is a Server Component and cannot directly use Client Components like `ThemeProvider` or `LanguageProvider` as JSX. Create a wrapper.

Create `web/components/providers.tsx`:

```tsx
"use client";

import { ThemeProvider } from "next-themes";
import { LanguageProvider } from "@/lib/i18n/context";
import type { ReactNode } from "react";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      storageKey="changelogue-theme"
    >
      <LanguageProvider>{children}</LanguageProvider>
    </ThemeProvider>
  );
}
```

- [ ] **Step 2: Update layout.tsx**

Replace the import:
```tsx
import { AuthProvider } from "@/lib/auth/context";
```

Add after it:
```tsx
import { Providers } from "@/components/providers";
```

Change the `<html>` tag from:
```tsx
    <html lang="en">
```
to:
```tsx
    <html lang="en" suppressHydrationWarning>
```

Change the body content from:
```tsx
        <AuthProvider>
          <LayoutShell>{children}</LayoutShell>
        </AuthProvider>
```
to:
```tsx
        <Providers>
          <AuthProvider>
            <LayoutShell>{children}</LayoutShell>
          </AuthProvider>
        </Providers>
```

- [ ] **Step 3: Verify the dev server starts**

```bash
cd web && npx next build 2>&1 | tail -5
```
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add web/components/providers.tsx web/app/layout.tsx
git commit -m "feat: wire ThemeProvider and LanguageProvider into root layout via Providers wrapper"
```

---

### Task 5: Create the Settings Dialog component

**Files:**
- Create: `web/components/settings/settings-dialog.tsx`

- [ ] **Step 1: Create the settings dialog**

Create `web/components/settings/settings-dialog.tsx`:

```tsx
"use client";

import { useTheme } from "next-themes";
import { Sun, Moon, Monitor } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n/context";
import { cn } from "@/lib/utils";

interface SettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function SettingsDialog({ open, onOpenChange }: SettingsDialogProps) {
  const { theme, setTheme } = useTheme();
  const { t, locale, setLocale } = useTranslation();

  const themeOptions = [
    { value: "light", label: t("settings.theme.light"), icon: Sun },
    { value: "dark", label: t("settings.theme.dark"), icon: Moon },
    { value: "system", label: t("settings.theme.system"), icon: Monitor },
  ] as const;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>{t("settings.title")}</DialogTitle>
        </DialogHeader>

        <div className="space-y-6 pt-2">
          {/* Theme */}
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("settings.theme")}</label>
            <div className="flex gap-2">
              {themeOptions.map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => setTheme(opt.value)}
                  className={cn(
                    "flex flex-1 items-center justify-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors",
                    theme === opt.value
                      ? "border-[var(--beacon-accent)] bg-[var(--beacon-accent)]/10 text-[var(--beacon-accent)]"
                      : "border-border hover:bg-accent"
                  )}
                >
                  <opt.icon className="h-4 w-4" />
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          {/* Language */}
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("settings.language")}</label>
            <Select value={locale} onValueChange={(v) => setLocale(v as "en" | "zh")}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="en">{t("settings.language.en")}</SelectItem>
                <SelectItem value="zh">{t("settings.language.zh")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd web && npx tsc --noEmit 2>&1 | grep -i "settings" || echo "No settings errors"
```
Expected: `No settings errors`

- [ ] **Step 3: Commit**

```bash
git add web/components/settings/
git commit -m "feat: add SettingsDialog component with theme and language controls"
```

---

### Task 6: Update sidebar with DropdownMenu and settings trigger

**Files:**
- Modify: `web/components/layout/sidebar.tsx`

- [ ] **Step 1: Add imports**

Add these imports to the top of `sidebar.tsx`:

```tsx
import { Settings } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SettingsDialog } from "@/components/settings/settings-dialog";
import { useTranslation } from "@/lib/i18n/context";
```

- [ ] **Step 2: Add state and translation hook inside the Sidebar component**

After the existing `const { user, logout } = useAuth();` line, add:

```tsx
  const [settingsOpen, setSettingsOpen] = useState(false);
  const { t } = useTranslation();
```

- [ ] **Step 3: Refactor sidebar `<aside>` background to use CSS variable**

Change:
```tsx
      style={{ backgroundColor: "#16181c" }}
```
to:
```tsx
      style={{ backgroundColor: "var(--sidebar-bg)" }}
```

- [ ] **Step 4: Update nav items to use translation keys**

Replace the module-level `navItems` const (stays at module scope — only contains string keys, not translated values):

```tsx
const navKeys = [
  { href: "/", key: "nav.dashboard", icon: LayoutDashboard },
  { href: "/projects", key: "nav.projects", icon: FolderKanban },
  { href: "/todo", key: "nav.todo", icon: ListTodo },
  { href: "/releases", key: "nav.releases", icon: Package },
  { href: "/channels", key: "nav.channels", icon: Megaphone },
  { href: "/subscriptions", key: "nav.subscriptions", icon: Bell },
];
```

In the nav mapping JSX, update the iteration and references (note: `useState` is already imported):

Change:
```tsx
        {navItems.map((item) => {
```
to:
```tsx
        {navKeys.map((item) => {
```

Change the `title` attribute from:
```tsx
              title={expanded ? undefined : item.label}
```
to:
```tsx
              title={expanded ? undefined : t(item.key)}
```

Change the `<span>` text from:
```tsx
                <span style={{ fontFamily: "var(--font-dm-sans)" }}>
                  {item.label}
                </span>
```
to:
```tsx
                <span style={{ fontFamily: "var(--font-dm-sans)" }}>
                  {t(item.key)}
                </span>
```

- [ ] **Step 5: Replace the user section with DropdownMenu**

Replace the entire `{/* User section */}` block (lines 113-142) with:

```tsx
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
                  <span className="truncate text-xs text-[#9ca3af]">
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
```

- [ ] **Step 6: Verify TypeScript compiles**

```bash
cd web && npx tsc --noEmit 2>&1 | grep -i "sidebar" || echo "No sidebar errors"
```
Expected: `No sidebar errors`

- [ ] **Step 7: Commit**

```bash
git add web/components/layout/sidebar.tsx
git commit -m "feat: add settings dropdown to sidebar user menu with i18n labels"
```

---

### Task 7: Manual verification

- [ ] **Step 1: Start dev server**

```bash
cd web && npm run dev
```

- [ ] **Step 2: Verify theme toggle**

Open the app, click the user avatar in the sidebar, click "Settings". Toggle between Light/Dark/System. Verify:
- Light mode: white background, dark text
- Dark mode: dark background, light text, cards and popovers use dark surface colors
- System mode: follows OS preference
- Refresh page — theme persists

- [ ] **Step 3: Verify language toggle**

In the settings dialog, switch language to 中文. Verify:
- Sidebar nav labels change to Chinese
- Settings dialog labels change to Chinese
- Refresh page — language persists
- Switch back to English — all labels revert

- [ ] **Step 4: Verify collapsed sidebar**

Collapse the sidebar. Click the user avatar. Verify the dropdown menu opens to the right side and both Settings and Sign out items are accessible.

- [ ] **Step 5: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: address issues found during manual verification"
```
