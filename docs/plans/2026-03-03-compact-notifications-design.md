# Compact Two-Tier Notifications Design

## Problem

Semantic release notifications (Slack/Discord) are too verbose — they dump every report field (risk, urgency, availability, adoption, status checks, changelog, download commands, download links, recommendation) with dividers between each, making them hard to scan quickly.

Additionally, `risk_level` and `urgency` are near-duplicates that usually hold the same value.

## Goals

- Notifications should be glanceable (5-6 lines)
- Surface: urgency level, what changed, download command
- Add urgency reason detail only for Critical/High urgency
- Consolidate `risk_level`/`risk_reason` into `urgency`/`urgency_reason`
- Full details remain available via "View in Changelogue" link

## Design

### Schema Change

Replace `risk_level` + `risk_reason` with `urgency_reason`. Keep `urgency` as the primary severity field.

```go
type SemanticReport struct {
    Subject          string   `json:"subject"`
    Urgency          string   `json:"urgency"`          // Critical|High|Medium|Low
    UrgencyReason    string   `json:"urgency_reason"`   // Why this urgency level
    StatusChecks     []string `json:"status_checks"`
    ChangelogSummary string   `json:"changelog_summary"`
    DownloadCommands []string `json:"download_commands,omitempty"`
    DownloadLinks    []string `json:"download_links,omitempty"`
    Summary          string   `json:"summary,omitempty"`
    Availability     string   `json:"availability"`
    Adoption         string   `json:"adoption"`
    Recommendation   string   `json:"recommendation"`
    // Backward compat — old reports still have these
    RiskLevel        string   `json:"risk_level,omitempty"`
    RiskReason       string   `json:"risk_reason,omitempty"`
}
```

### LLM Prompt Update

Replace `risk_level` and `risk_reason` with `urgency` and `urgency_reason` in `DefaultInstruction`.

### Slack Notification — Two-Tier Format

**Low/Medium urgency (compact):**
```
📦 Geth v1.10.15 — 🟢 Low Urgency
Fixes sync bug in block 14,000,000
docker pull ethereum/client-go:v1.10.15
GitHub · ethereum/go-ethereum  |  View on GitHub  |  View in Changelogue
```

**Critical/High urgency (adds reason):**
```
📦 Geth v1.10.15 — ⚫ Critical Urgency
⚠️ Hard Fork detected — deploy before block 18,000,000
Fixes sync bug in block 14,000,000
docker pull ethereum/client-go:v1.10.15
GitHub · ethereum/go-ethereum  |  View on GitHub  |  View in Changelogue
```

Block Kit structure:
1. Header — `{ProjectName} {Version} — {emoji} {Urgency} Urgency`
2. Section (conditional, Critical/High only) — urgency reason
3. Section — changelog summary
4. Section (conditional) — first download command as code
5. Context — provider info + links

**Dropped from notifications:** subject, risk_level, availability, adoption, recommendation, status checks, extra download commands/links.

### Discord Notification — Two-Tier Format

Same logic using Discord embed:
- Title: `{ProjectName} {Version}`
- Description: urgency badge + optional reason + changelog + download command + links
- Color: derived from urgency (Critical=near-black, High=red, Medium=amber, Low=green)
- No inline fields (removes risk/urgency/availability/adoption fields)
- Footer: provider + repository

### Webhook Payload

No changes — continues sending full structured SemanticReport JSON.

### Frontend Detail Page

Update to use `urgency`/`urgency_reason` instead of `risk_level`/`risk_reason` with fallback to old fields for backward compatibility.

### Backward Compatibility

Old JSONB reports in the database still contain `risk_level` and `risk_reason`. The struct retains these as `omitempty`. Display code falls back: use `urgency` if present, else `risk_level`; use `urgency_reason` if present, else `risk_reason`.

## Files Changed

1. `internal/models/semantic_release.go` — update SemanticReport struct
2. `internal/agent/orchestrator.go` — update DefaultInstruction prompt
3. `internal/routing/slack.go` — rewrite buildSemanticBlocks()
4. `internal/routing/discord.go` — rewrite buildSemanticEmbed(), discordRiskColor()
5. `internal/routing/sender.go` — rename riskEmoji() to urgencyEmoji()
6. `web/lib/api/types.ts` — update SemanticReport TypeScript type
7. `web/components/semantic-releases/semantic-release-detail.tsx` — use urgency/urgency_reason with fallback
8. Test files for affected packages

## Not Changing

- Database schema (JSONB is schema-less)
- API response format (struct change is additive)
- Webhook payload structure
- Agent tools or sub-agents
