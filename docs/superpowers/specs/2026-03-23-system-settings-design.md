# System Settings: Theme & Language

## Summary

Add a system settings feature to Changelogue that lets users switch between light/dark/system theme and English/Chinese language. Settings are accessed from the sidebar user menu and displayed in a modal dialog. All preferences persist in localStorage (per-device).

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Entry point | Sidebar user menu dropdown | Non-intrusive, near existing user profile section |
| Settings UI | Modal dialog | Lightweight for 2 settings, no page navigation needed |
| Theme options | Light / Dark / System | System follows OS `prefers-color-scheme`, default |
| Languages | English + Chinese (Simplified) | Two languages to start |
| Persistence | localStorage | No backend changes, per-device, simple |
| Theme library | next-themes | Handles `.dark` class, flash prevention, 3KB |
| i18n approach | Custom React context | Zero deps, simple for 2 languages, flat JSON keys |

## Architecture

```
Providers (root layout, nesting order matters)
ThemeProvider (outermost — sets class on <html>)
└── LanguageProvider
    └── AuthProvider (existing, innermost)

Sidebar user menu
└── DropdownMenu
    ├── Settings → opens SettingsDialog
    └── Sign out (existing)

SettingsDialog (modal)
├── Theme — Light/Dark/System toggle buttons (sun/moon/monitor icons)
└── Language — Dropdown select (English / 中文)
```

## New Files

### `web/lib/i18n/context.tsx`
- `LanguageProvider` — reads `locale` from localStorage key `"changelogue-locale"`, default `en`
- On locale change, updates `document.documentElement.lang` to keep `<html lang>` in sync
- `useTranslation()` hook — returns `{ t, locale, setLocale }`
- `t(key)` — looks up translation string from current locale's messages. Falls back to English string if key missing in current locale. Returns the key itself if missing from all locales.

### `web/lib/i18n/messages/en.json`
- Flat dot-notation keys: `"nav.dashboard": "Dashboard"`, etc.
- Covers: navigation labels, settings UI, page titles

### `web/lib/i18n/messages/zh.json`
- Same keys, Chinese translations

### `web/components/settings/settings-dialog.tsx`
- Modal dialog with two sections (no tabs)
- Theme: three toggle buttons with icons (Sun/Moon/Monitor)
- Language: Select dropdown (English / 中文)
- Uses existing shadcn Dialog, Select components
- Theme controlled via `next-themes` `useTheme()`
- Language controlled via custom `useTranslation()`

## Modified Files

### `web/app/layout.tsx`
- Add `suppressHydrationWarning` to `<html>` element (required by next-themes)
- Wrap children: `<ThemeProvider><LanguageProvider><AuthProvider>...</AuthProvider></LanguageProvider></ThemeProvider>`
- ThemeProvider config: `attribute="class"`, `defaultTheme="system"`, `enableSystem`, `storageKey="changelogue-theme"`

### `web/app/globals.css`
- Add `html.dark` selector block (plain CSS, not Tailwind variant) with dark mode CSS variables:
  - `--background: #0f1115`
  - `--surface: #16181c`
  - `--foreground: #e5e7eb`
  - `--border: #2a2d33`
  - `--sidebar-bg: #0a0b0e`
  - `--text-muted: #6b7280` (same)
  - `--beacon-accent: #e8601a` (same)
  - Plus all shadcn token overrides (primary, secondary, card, popover, etc.)

### `web/components/layout/sidebar.tsx`
- Replace hardcoded user section with DropdownMenu
- Menu items: "Settings" (opens SettingsDialog), "Sign out" (existing)
- Import and render SettingsDialog
- Refactor hardcoded inline color styles (e.g., `backgroundColor: "#16181c"`) to use CSS variables so dark mode applies correctly
- In collapsed mode (52px), the avatar serves as the DropdownMenu trigger with `side="right"` positioning

### `web/package.json`
- Add `next-themes` dependency

## Dark Mode Color Tokens

| Token | Light | Dark |
|-------|-------|------|
| --background | #fafaf9 | #0f1115 |
| --surface | #ffffff | #16181c |
| --foreground | #111113 | #e5e7eb |
| --border | #e8e8e5 | #2a2d33 |
| --sidebar-bg | #16181c | #0a0b0e |
| --beacon-accent | #e8601a | #e8601a |
| --text-muted | #6b7280 | #6b7280 |

## i18n Message Keys (initial set)

Translations cover navigation, settings UI, and page titles. All other UI text remains English initially and gets translated incrementally.

```json
{
  "nav.dashboard": "Dashboard",
  "nav.projects": "Projects",
  "nav.todo": "Todo",
  "nav.releases": "Releases",
  "nav.channels": "Channels",
  "nav.subscriptions": "Subscriptions",
  "settings.title": "Settings",
  "settings.appearance": "Appearance",
  "settings.theme": "Theme",
  "settings.theme.light": "Light",
  "settings.theme.dark": "Dark",
  "settings.theme.system": "System",
  "settings.language": "Language",
  "settings.language.en": "English",
  "settings.language.zh": "中文",
  "user.signout": "Sign out"
}
```

## Out of Scope

- Backend persistence (user_preferences API)
- RTL language support
- Additional languages beyond EN/ZH
- Translating dynamic content (release notes, agent reports)
- Per-page/component translation beyond nav, settings, and page titles
- Refactoring all hardcoded colors across the codebase (components outside sidebar that use inline hex colors or Tailwind arbitrary values like `bg-[#f3f3f1]` will not respond to dark mode — these get fixed incrementally in follow-up work)
- Dark mode overrides for `.release-notes-content` CSS block
- Login/onboard page dark mode styling (these pages use hardcoded colors and will be addressed separately)
