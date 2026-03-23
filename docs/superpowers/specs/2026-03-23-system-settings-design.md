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
Providers (root layout)
├── AuthProvider (existing)
├── ThemeProvider (next-themes)
└── LanguageProvider (custom context)

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
- `LanguageProvider` — reads `locale` from localStorage, default `en`
- `useTranslation()` hook — returns `{ t, locale, setLocale }`
- `t(key)` — looks up translation string from current locale's messages

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
- Wrap children with `ThemeProvider` (from next-themes) and `LanguageProvider`
- ThemeProvider config: `attribute="class"`, `defaultTheme="system"`, `enableSystem`

### `web/app/globals.css`
- Add `.dark` selector block with dark mode CSS variables:
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
