# Project Logos Design

## Summary

Show project logos on the projects list page and project detail page. The logo is resolved from the project's sources using a priority-based approach, with GitHub organization/user avatars as the primary source.

## Logo Resolution

### Priority Order

When a project has multiple sources, pick the best one: **GitHub > GitLab > Docker Hub > ECR**

### Avatar URL Construction (Client-Side)

- **GitHub**: `https://github.com/{owner}.png?size=64` — owner extracted from `repository` field (`owner/repo` → `owner`)
- **GitLab / Docker Hub / ECR**: No reliable public avatar URL pattern — fall back to placeholder

### Placeholder (Fallback)

When no avatar is available or the image fails to load:
- Colored circle with the **first letter** of the project name
- Color derived deterministically from the project name (hash → hue) for consistent, distinct colors per project

## Component: `ProjectLogo`

**Props:**
- `project: Project` — for name (placeholder text + color)
- `sources: Source[]` — for resolving avatar URL
- `size: number` — pixel size (default 40)

**Sizes by context:**
- Card view: 40px
- Compact view: 24px
- Detail page header: 48px

## Integration Points

1. **Projects list page, card view** — logo next to project name in card header
2. **Projects list page, compact view** — small logo in name column
3. **Project detail page** — logo next to project name in header

## Technical Notes

- Pure client-side — no backend changes needed
- Uses Next.js `<Image>` or `<img>` with `onError` handler to switch to placeholder on load failure
- Deterministic color from project name ensures consistency across page loads
