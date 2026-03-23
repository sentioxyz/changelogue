# Session Context

## User Prompts

### Prompt 1

Implement the following plan:

# Fix Dark Mode & i18n Across All Pages

## Context

After implementing the settings dialog with theme toggle (light/dark/system) and language selector (EN/ZH), users reported:
1. **Dark mode broken**: Cards, buttons, tables still show white/light backgrounds because components use hardcoded hex colors instead of CSS variables
2. **i18n incomplete**: Only sidebar nav labels are translated — all page content (titles, tabs, buttons, table headers, messages) remains...

### Prompt 2

I see, thanks. Two things: 1. the chinese font is incosistent with english one, 2. in the projects page, if the source with error like failed to poll, the dark mode style is not right

### Prompt 3

The localization for pop up settings like source, subscription etc. still wrong

### Prompt 4

This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.

Analysis:
Let me chronologically analyze the conversation:

1. **Initial Plan**: The user provided a detailed plan to fix dark mode and i18n across ~30+ files in a Next.js web app called ReleaseBeacon/Changelogue. The plan included:
   - Color mapping table (hardcoded hex → CSS variables)
   - i18n strategy (add keys to en.json/zh.json, use `t...

### Prompt 5

cool, commit and push

### Prompt 6

Calling setState synchronously within an effect body causes cascading renders that can hurt performance, and is not recommended. (https://react.dev/learn/you-might-not-need-an-effect).

/home/runner/work/changelogue/changelogue/web/lib/i18n/context.tsx:35:7
  33 |     const stored = localStorage.getItem(STORAGE_KEY);
  34 |     if (stored === "en" || stored === "zh") {
> 35 |       setLocaleState(stored);
     |       ^^^^^^^^^^^^^^ Avoid calling setState() directly within an effect
  36 |      ...

