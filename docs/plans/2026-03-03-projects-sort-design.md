# Projects Page Sort Design

## Overview

Add client-side sorting to the projects listing page with three sort options: Recently updated, Recently added, and Name.

## Sort Options

| Label | Field | Direction |
|-------|-------|-----------|
| Recently updated | `updated_at` | DESC |
| Recently added | `created_at` | DESC |
| Name | `name` | ASC |

Default: **Recently updated**

## Approach

Client-side sorting. Projects are already fetched in bulk (page=1, perPage=100), so sorting happens in the frontend on the filtered array before pagination. No backend changes needed.

## UI

- Sort dropdown placed next to the existing search bar
- Matches existing UI style (view toggle pattern)
- State: `sortBy` with values `"updated" | "added" | "name"`

## Files Changed

- `web/app/projects/page.tsx` — add sort state, sort dropdown, sort logic
