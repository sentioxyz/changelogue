# Projects Page: Show Recent Releases

**Date:** 2026-02-26
**Status:** Approved

## Problem

The projects list page currently shows Agent Rules badges (major/minor/security) which are not very useful at a glance. Users want to see recent release activity per project instead.

## Design

### Approach: Frontend-driven fetching

- **No backend changes.** Use existing endpoints:
  - `GET /projects/{id}/releases?per_page=3`
  - `GET /projects/{id}/semantic-releases?per_page=3`
- Each project row fetches its own releases via SWR (cached, deduped)
- Show up to 3 version chips initially with a "more..." link to the project detail page

### Table Layout

| Name | Description | Recent Releases | Semantic Releases | Created |

### Changes

1. **Extract `ProjectRow` component** — each row calls SWR for releases and semantic releases independently
2. **Remove Agent Rules column** — replaced by two release columns
3. **Recent Releases column** — up to 3 clickable `VersionChip` wrapped in `Link` to `/releases/{id}`
4. **Semantic Releases column** — up to 3 clickable `VersionChip` wrapped in `Link` to `/projects/{projectId}/semantic-releases/{srId}`
5. **"more..." link** — shown when releases exist beyond displayed count, links to the project detail page's relevant tab

### Extensibility

- Change `per_page=3` to any number to show more
- Could add inline "load more" pagination later
- No backend coupling to a specific limit

### Existing Components

- `VersionChip` (`web/components/ui/version-chip.tsx`) — non-interactive span, will be wrapped in `Link`
- Release detail page: `/releases/{id}`
- Semantic release detail page: `/projects/{projectId}/semantic-releases/{srId}`
