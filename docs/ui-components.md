# Shared UI Components Reference

Reference for all unified UI components in `web/components/`. Use these instead of creating inline alternatives.

## Core Components (`web/components/ui/`)

### UrgencyPill

Unified urgency indicator. **Do not** create local urgency colors or chips — always use this.

```tsx
import { UrgencyPill } from "@/components/ui/urgency-pill";

// Three variants:
<UrgencyPill urgency="critical" variant="icon-only" />  // 18×18 circle icon (compact spaces)
<UrgencyPill urgency="high" variant="labeled" />         // icon + text pill (tables, detail views)
<UrgencyPill urgency="medium" variant="text" />           // text-only pill (feeds, lists)
```

| Props | Type | Default | Description |
|-------|------|---------|-------------|
| `urgency` | `string` | — | `critical`, `high`, `medium`, `low` (case-insensitive) |
| `variant` | `"icon-only" \| "labeled" \| "text"` | `"labeled"` | Display format |
| `className` | `string?` | — | Additional classes |

Levels: critical (red), high (orange), medium (amber), low (green).

### UrgencyCallout

Alert-style block for HIGH/CRITICAL urgency on detail pages. Renders nothing for medium/low.

```tsx
import { UrgencyCallout } from "@/components/ui/urgency-callout";

<UrgencyCallout urgency="critical" description="Breaking API changes detected" />
```

| Props | Type | Default | Description |
|-------|------|---------|-------------|
| `urgency` | `string` | — | Only renders for `high` or `critical` |
| `description` | `string?` | — | Optional detail text |
| `className` | `string?` | — | Additional classes |

### Switch

Radix UI toggle for on/off states (enable/disable polling, feature toggles).

```tsx
import { Switch } from "@/components/ui/switch";

<Switch checked={enabled} onCheckedChange={setEnabled} size="sm" />
```

| Props | Type | Default | Description |
|-------|------|---------|-------------|
| `checked` | `boolean` | — | Current state |
| `onCheckedChange` | `(checked: boolean) => void` | — | Toggle handler |
| `size` | `"sm" \| "default"` | `"default"` | Switch size |

### Checkbox

Radix UI checkbox for multi-select and boolean fields.

```tsx
import { Checkbox } from "@/components/ui/checkbox";

<Checkbox checked={value} onCheckedChange={(checked) => setValue(!!checked)} />
```

Standard Radix `CheckboxPrimitive.Root` props.

### VersionChip

Monospaced version label in a compact pill.

```tsx
import { VersionChip } from "@/components/ui/version-chip";

<VersionChip version="v2.1.0" />
```

| Props | Type | Default | Description |
|-------|------|---------|-------------|
| `version` | `string` | — | Version string to display |
| `className` | `string?` | — | Additional classes |

### ProviderBadge

Registry provider badge with icon and label.

```tsx
import { ProviderBadge, getProviderIcon } from "@/components/ui/provider-badge";

<ProviderBadge provider="github" />

// Icon only (e.g. in feed rows):
const Icon = getProviderIcon("dockerhub");
// Returns a render function: (props: { size, className }) => JSX
```

Supported providers: `github`, `dockerhub`, `ecr-public`, `gitlab`, `pypi`, `npm`.

### StatusDot

2×2px colored dot for status indicators.

```tsx
import { StatusDot } from "@/components/ui/status-dot";

<StatusDot status="completed" />
```

Status colors: `completed` (green), `running` (blue), `pending` (orange), `failed` (red). Unknown statuses render gray.

### ProjectLogo

Avatar that prefers GitHub/GitLab avatar, falls back to a colored initial.

```tsx
import { ProjectLogo } from "@/components/ui/project-logo";

<ProjectLogo name="my-project" sources={project.sources} size={40} />
```

| Props | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | `string` | — | Project name (used for initial + color hash) |
| `sources` | `Source[]?` | — | Project sources (for GitHub/GitLab avatar lookup) |
| `size` | `number` | `40` | Pixel size |

### SectionLabel

Uppercase muted section heading.

```tsx
import { SectionLabel } from "@/components/ui/section-label";

<SectionLabel>Trigger Rules</SectionLabel>
```

### ConfirmDialog

Modal confirmation with async action support.

```tsx
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

<ConfirmDialog
  open={showDelete}
  onOpenChange={setShowDelete}
  title="Delete release?"
  description="This action cannot be undone."
  onConfirm={async () => { await deleteRelease(id); }}
  confirmLabel="Delete"  // optional, defaults to i18n label
/>
```

### Button

Primary action button with CVA variants.

```tsx
import { Button } from "@/components/ui/button";

<Button variant="default" size="sm">Save</Button>
<Button variant="destructive" size="icon"><Trash2 /></Button>
```

Variants: `default`, `destructive`, `outline`, `secondary`, `ghost`, `link`.
Sizes: `default`, `xs`, `sm`, `lg`, `icon`, `icon-xs`, `icon-sm`, `icon-lg`.

### Badge

Inline badge/tag component.

```tsx
import { Badge } from "@/components/ui/badge";

<Badge variant="secondary">Beta</Badge>
```

Variants: `default`, `secondary`, `destructive`, `outline`, `ghost`, `link`.

### Card

Composable card layout.

```tsx
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter, CardAction } from "@/components/ui/card";

<Card>
  <CardHeader>
    <CardTitle>Title</CardTitle>
    <CardDescription>Subtitle</CardDescription>
    <CardAction><Button>Edit</Button></CardAction>
  </CardHeader>
  <CardContent>...</CardContent>
</Card>
```

### Input

Styled text input with error state support.

```tsx
import { Input } from "@/components/ui/input";

<Input placeholder="Search..." aria-invalid={!!error} />
```

### Table

Composable semantic table.

```tsx
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";

<Table>
  <TableHeader>
    <TableRow>
      <TableHead>Name</TableHead>
    </TableRow>
  </TableHeader>
  <TableBody>
    <TableRow>
      <TableCell>Value</TableCell>
    </TableRow>
  </TableBody>
</Table>
```

## Filter Components (`web/components/filters/`)

### FilterBar

Chip-based filter UI with popover menus. Supports select, boolean, and date-range filter types.

```tsx
import { FilterBar } from "@/components/filters/filter-bar";
import type { FilterConfig } from "@/components/filters/filter-bar";

const filters: FilterConfig[] = [
  { key: "provider", label: "Provider", type: "select", options: [
    { value: "github", label: "GitHub" },
    { value: "dockerhub", label: "Docker Hub" },
  ]},
  { key: "excluded", label: "Excluded", type: "boolean", defaultValue: "false" },
  { key: "date", label: "Date", type: "date-range" },
];

<FilterBar filters={filters} value={currentFilters} onChange={setFilters} />
```

Date presets: 7d, 30d, 90d, 1y. Use `expandDatePreset()` to convert to ISO date strings for API calls.

### useFilterParams

Hook that syncs filter + pagination state with URL query parameters. Prevents cross-page param leakage.

```tsx
import { useFilterParams } from "@/components/filters/use-filter-params";

const { filters, setFilters, page, setPage } = useFilterParams(
  ["provider", "excluded", "date_from", "date_to"],  // allowed URL keys
  { excluded: "false" }                                // defaults
);
```

Only keys in `allowedKeys` are read from or written to the URL. Stale params from other pages are ignored.

## Utility Functions (`web/lib/`)

### timeAgo (`web/lib/format.ts`)

```tsx
import { timeAgo } from "@/lib/format";

timeAgo("2026-03-29T10:00:00Z")  // "2h ago"
timeAgo(null)                     // "—"
```

### formatInterval (`web/lib/format.ts`)

```tsx
import { formatInterval } from "@/lib/format";

formatInterval(3600)   // "Hourly"
formatInterval(86400)  // "Daily"
```

### useTranslation (`web/lib/i18n/context`)

```tsx
import { useTranslation } from "@/lib/i18n/context";

const { t } = useTranslation();
<span>{t("sr.title")}</span>
```

Messages in `web/lib/i18n/messages/en.json` and `zh.json`.

## Design System Constants

### Urgency Levels
| Level | Color | Icon |
|-------|-------|------|
| critical | Red (`#dc2626`) | `AlertOctagon` |
| high | Orange (`#ea580c`) | `AlertTriangle` |
| medium | Amber (`#d97706`) | `Circle` |
| low | Green (`#16a34a`) | `CheckCircle` |

### Status Colors
| Status | Color |
|--------|-------|
| completed | Green (`#16a34a`) |
| running | Blue (`#2563eb`) |
| pending | Orange (`#d97706`) |
| failed | Red (`#dc2626`) |

### Fonts
- **Headings:** `var(--font-raleway)` — Raleway
- **Body text:** `var(--font-dm-sans)` — DM Sans
- **Code/versions:** `'JetBrains Mono', monospace`

### Icons
Use **Lucide React** (`lucide-react`) for all icons. Provider-specific icons use **react-icons/fa** (Font Awesome).

### CSS Variables
Theme colors are defined as CSS variables: `--foreground`, `--background`, `--surface`, `--border`, `--text-secondary`, `--text-muted`, `--beacon-accent` (`#e8601a`), `--status-completed`.
