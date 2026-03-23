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

