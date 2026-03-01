# Pagination & Search for Projects Page

## Overview

Add client-side search and pagination to the projects page. Fetch all projects (up to 100), filter by name, display 12 per page.

## Search Box

- Text input with Search (Lucide) icon in header row
- Filters projects by name (case-insensitive substring match)
- Typing resets pagination to page 1

## Pagination

- 12 projects per page
- "Previous / Page X of Y / Next" controls at bottom of list
- Disabled buttons at boundaries

## Data Flow

- Fetch all with `projects.list(1, 100)`
- Client-side filter: `items.filter(p => p.name.toLowerCase().includes(query))`
- Client-side slice: `filtered.slice((page-1)*12, page*12)`
- `useState` for search and page — no persistence

## Implementation

All changes in `web/app/projects/page.tsx`. No backend changes.
