# Compact Two-Tier Notifications Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace verbose Slack/Discord notifications with compact two-tier format, consolidate risk_level/urgency into unified urgency field.

**Architecture:** Schema change (add urgency_reason, keep risk_level/risk_reason for backward compat), rewrite notification formatters to compact format, update LLM prompt, update frontend to prefer new fields with fallback.

**Tech Stack:** Go (backend), Next.js/React/TypeScript (frontend), ADK-Go (agent prompt)

---

### Task 1: Update SemanticReport struct

**Files:**
- Modify: `internal/models/semantic_release.go:8-24`
- Modify: `internal/models/semantic_release_test.go`

**Step 1: Add UrgencyReason field to SemanticReport**

In `internal/models/semantic_release.go`, add `UrgencyReason` and mark old risk fields as backward-compat:

```go
type SemanticReport struct {
	// Primary fields
	Subject          string   `json:"subject"`
	Urgency          string   `json:"urgency"`
	UrgencyReason    string   `json:"urgency_reason,omitempty"`
	StatusChecks     []string `json:"status_checks"`
	ChangelogSummary string   `json:"changelog_summary"`
	DownloadCommands []string `json:"download_commands,omitempty"`
	DownloadLinks    []string `json:"download_links,omitempty"`

	// Existing fields
	Summary        string `json:"summary,omitempty"`
	Availability   string `json:"availability"`
	Adoption       string `json:"adoption"`
	Recommendation string `json:"recommendation"`

	// Backward compat — old reports may still have these in JSONB
	RiskLevel  string `json:"risk_level,omitempty"`
	RiskReason string `json:"risk_reason,omitempty"`
}
```

**Step 2: Update the test to use UrgencyReason**

In `internal/models/semantic_release_test.go`, update the test fixture:

```go
func TestSemanticReportJSON(t *testing.T) {
	report := SemanticReport{
		Subject:          "Ready to Deploy: Geth v1.10.15 (Critical Update)",
		Urgency:          "Critical",
		UrgencyReason:    "Hard Fork detected in Discord #announcements",
		StatusChecks:     []string{"Docker Image Verified", "Binaries Available"},
		ChangelogSummary: "Fixes sync bug in block 14,000,000",
		Availability:     "GA",
		Adoption:         "12% of network updated",
		Recommendation:   "Wait for 25% adoption unless urgent.",
		DownloadCommands: []string{"docker pull ethereum/client-go:v1.10.15"},
		DownloadLinks:    []string{"https://github.com/ethereum/go-ethereum/releases/tag/v1.10.15"},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SemanticReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Subject != report.Subject {
		t.Errorf("subject: got %q, want %q", decoded.Subject, report.Subject)
	}
	if decoded.Urgency != report.Urgency {
		t.Errorf("urgency: got %q, want %q", decoded.Urgency, report.Urgency)
	}
	if decoded.UrgencyReason != report.UrgencyReason {
		t.Errorf("urgency_reason: got %q, want %q", decoded.UrgencyReason, report.UrgencyReason)
	}
	if len(decoded.StatusChecks) != 2 {
		t.Errorf("status_checks: got %d items, want 2", len(decoded.StatusChecks))
	}
	if len(decoded.DownloadCommands) != 1 {
		t.Errorf("download_commands: got %d items, want 1", len(decoded.DownloadCommands))
	}
	if len(decoded.DownloadLinks) != 1 {
		t.Errorf("download_links: got %d items, want 1", len(decoded.DownloadLinks))
	}
}
```

Also add a backward-compat test for old format reports:

```go
func TestSemanticReportJSON_BackwardCompat(t *testing.T) {
	// Old-format JSON stored in database with risk_level/risk_reason
	oldJSON := `{
		"subject": "Ready to Deploy: Geth v1.10.15",
		"risk_level": "CRITICAL",
		"risk_reason": "Hard Fork detected",
		"status_checks": ["Docker Image Verified"],
		"changelog_summary": "Fixes sync bug",
		"availability": "GA",
		"adoption": "12% updated",
		"urgency": "Critical",
		"recommendation": "Wait for 25% adoption."
	}`

	var report SemanticReport
	if err := json.Unmarshal([]byte(oldJSON), &report); err != nil {
		t.Fatalf("unmarshal old format: %v", err)
	}
	if report.RiskLevel != "CRITICAL" {
		t.Errorf("risk_level: got %q, want CRITICAL", report.RiskLevel)
	}
	if report.RiskReason != "Hard Fork detected" {
		t.Errorf("risk_reason: got %q", report.RiskReason)
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/models/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/models/semantic_release.go internal/models/semantic_release_test.go
git commit -m "refactor(models): add urgency_reason, keep risk fields for backward compat"
```

---

### Task 2: Update LLM agent prompt

**Files:**
- Modify: `internal/agent/orchestrator.go:46-76` (DefaultInstruction)
- Modify: `internal/agent/orchestrator_test.go`

**Step 1: Update DefaultInstruction**

Replace `risk_level` and `risk_reason` with `urgency` and `urgency_reason` in the JSON schema within `DefaultInstruction`. The `urgency` field already exists — just remove `risk_level`/`risk_reason` from the prompt and add `urgency_reason`.

Change the JSON template in `DefaultInstruction` (lines 64-76) from:

```
{
  "subject": "Ready to Deploy: <Project> <Version> (<Risk Summary>)",
  "risk_level": "CRITICAL|HIGH|MEDIUM|LOW",
  "risk_reason": "Why this risk level (e.g., 'Hard Fork detected in Discord #announcements')",
  ...
  "urgency": "Critical|High|Medium|Low",
  ...
}
```

To:

```
{
  "subject": "Ready to Deploy: <Project> <Version> (<Urgency Summary>)",
  "urgency": "Critical|High|Medium|Low",
  "urgency_reason": "Why this urgency level (e.g., 'Hard Fork detected in Discord #announcements')",
  "status_checks": ["Docker Image Verified", "Binaries Available"],
  "changelog_summary": "One-line summary of key changes (e.g., 'Fixes sync bug in block 14,000,000')",
  "availability": "GA|RC|Beta",
  "adoption": "Percentage or recommendation (e.g., '12% of network updated (Wait recommended if not urgent)')",
  "recommendation": "Actionable 1-2 sentence recommendation for the SRE team",
  "download_commands": ["docker pull ethereum/client-go:v1.10.15"],
  "download_links": ["https://gethstore.blob.core.windows.net/builds/geth-linux-amd64-1.10.15-8be800ff.tar.gz", "https://github.com/ethereum/go-ethereum/releases/tag/v1.10.15"]
}
```

**Step 2: Update orchestrator tests**

In `internal/agent/orchestrator_test.go`, update `TestParseReport_NewFormat` test fixture:
- Replace `"risk_level": "CRITICAL"` with nothing (remove it)
- Replace `"risk_reason": "Hard Fork detected"` with `"urgency_reason": "Hard Fork detected"`
- Update assertions: replace `report.RiskLevel` check with `report.UrgencyReason` check

Update `TestParseReport_OldFormat` — this already has only `urgency`, no changes needed.

**Step 3: Run tests**

Run: `go test ./internal/agent/... -v -run TestParseReport`
Expected: PASS

Run: `go test ./internal/agent/... -v -run TestVersionPlaceholder`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/agent/orchestrator.go internal/agent/orchestrator_test.go
git commit -m "refactor(agent): replace risk_level/risk_reason with urgency_reason in prompt"
```

---

### Task 3: Rename riskEmoji to urgencyEmoji and update to use Urgency field

**Files:**
- Modify: `internal/routing/slack.go:40-54` (riskEmoji function)

**Step 1: Rename riskEmoji to urgencyEmoji**

The function is in `slack.go` but called from both `slack.go` and `discord.go`. Rename it and update the switch to handle both UPPERCASE (old risk_level format) and Capitalized (urgency format):

```go
// urgencyEmoji returns an emoji indicator for the given urgency level.
func urgencyEmoji(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "LOW":
		return "🟢"
	case "MEDIUM":
		return "🟡"
	case "HIGH":
		return "🔴"
	case "CRITICAL":
		return "⚫"
	default:
		return "⚪"
	}
}
```

**Step 2: Run tests to verify rename compiles**

Run: `go build ./internal/routing/...`
Expected: Compile errors in slack.go and discord.go (references to old name). These will be fixed in Tasks 4 and 5.

**Step 3: Commit** (do NOT commit yet — this rename is combined with Tasks 4 and 5)

---

### Task 4: Rewrite Slack buildSemanticBlocks to compact format

**Files:**
- Modify: `internal/routing/slack.go:57-178` (buildSemanticBlocks function)
- Modify: `internal/routing/slack_test.go`

**Step 1: Write the updated test for compact semantic blocks**

Replace `TestSlackSender_SemanticReport` in `slack_test.go` with a test for the new compact format:

```go
func TestSlackSender_SemanticReport(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := &SlackSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	reportJSON := `{
		"subject": "Ready to Deploy: go-ethereum v1.16.4",
		"urgency": "High",
		"urgency_reason": "Security vulnerability patched",
		"status_checks": ["Binaries Unverified", "Docker Image Unverified"],
		"changelog_summary": "Security fixes and performance improvements",
		"availability": "GA",
		"adoption": "Recommended for production",
		"recommendation": "Deploy after verifying checksums",
		"download_commands": ["docker pull ethereum/client-go:v1.16.4"],
		"download_links": ["https://github.com/ethereum/go-ethereum/releases/tag/v1.16.4"]
	}`

	msg := Notification{
		Title:       "Semantic Release Report: go-ethereum v1.16.4",
		Body:        reportJSON,
		Version:     "v1.16.4",
		ProjectName: "go-ethereum",
		Provider:    "github",
		Repository:  "ethereum/go-ethereum",
		SourceURL:   "https://github.com/ethereum/go-ethereum/releases/tag/v1.16.4",
		ReleaseURL:  "https://changelogue.example.com/sr/1",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload slackPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	// Compact format: header + urgency_reason (High) + changelog + download cmd + context = ~5 blocks
	if len(payload.Blocks) < 3 || len(payload.Blocks) > 6 {
		t.Fatalf("expected 3-6 blocks for compact semantic report, got %d", len(payload.Blocks))
	}

	// First block should be header containing project name, version, and urgency
	if payload.Blocks[0].Type != "header" {
		t.Fatalf("expected first block to be header, got %s", payload.Blocks[0].Type)
	}
	headerText := payload.Blocks[0].Text.Text
	if !strings.Contains(headerText, "go-ethereum") || !strings.Contains(headerText, "v1.16.4") {
		t.Fatalf("header should contain project name and version, got %q", headerText)
	}
	if !strings.Contains(headerText, "High") {
		t.Fatalf("header should contain urgency level, got %q", headerText)
	}

	// Should NOT have a fields section (no separate risk/urgency/availability fields)
	for _, b := range payload.Blocks {
		if len(b.Fields) > 0 {
			t.Fatal("compact format should not have field sections")
		}
	}

	// Should have download command in a code block
	hasCode := false
	for _, b := range payload.Blocks {
		if b.Text != nil && strings.Contains(b.Text.Text, "docker pull") {
			hasCode = true
		}
	}
	if !hasCode {
		t.Fatal("expected download command")
	}

	// Last block should be context footer
	lastBlock := payload.Blocks[len(payload.Blocks)-1]
	if lastBlock.Type != "context" {
		t.Fatalf("expected last block to be context, got %s", lastBlock.Type)
	}
}
```

Also add a test that LOW urgency does NOT include urgency_reason:

```go
func TestSlackSender_SemanticReport_LowUrgency(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := &SlackSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	reportJSON := `{
		"subject": "go-ethereum v1.16.5",
		"urgency": "Low",
		"urgency_reason": "Minor dependency update",
		"changelog_summary": "Bumped go-libp2p to v0.30.0",
		"availability": "GA",
		"download_commands": ["docker pull ethereum/client-go:v1.16.5"]
	}`

	msg := Notification{
		Title:       "Semantic Release Report: go-ethereum v1.16.5",
		Body:        reportJSON,
		Version:     "v1.16.5",
		ProjectName: "go-ethereum",
		Provider:    "github",
		Repository:  "ethereum/go-ethereum",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload slackPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	// Low urgency should NOT include urgency_reason block
	for _, b := range payload.Blocks {
		if b.Text != nil && strings.Contains(b.Text.Text, "Minor dependency update") {
			t.Fatal("low urgency should not include urgency_reason in notification")
		}
	}
}
```

**Step 2: Run tests to see them fail**

Run: `go test ./internal/routing/ -v -run TestSlackSender_SemanticReport`
Expected: FAIL (old format produces too many blocks, has fields sections)

**Step 3: Rewrite buildSemanticBlocks**

Replace the entire `buildSemanticBlocks` function in `slack.go` with:

```go
// buildSemanticBlocks builds compact Slack Block Kit blocks from a SemanticReport.
// For Critical/High urgency, includes the urgency reason. For Low/Medium, omits it.
func buildSemanticBlocks(title string, report *models.SemanticReport, msg Notification) []slackBlock {
	// Resolve urgency from new field or backward-compat risk_level
	urgency := report.Urgency
	if urgency == "" {
		urgency = report.RiskLevel
	}
	urgencyReason := report.UrgencyReason
	if urgencyReason == "" {
		urgencyReason = report.RiskReason
	}

	// Header: "ProjectName vX.Y.Z — 🟢 Low Urgency"
	headerText := title
	if urgency != "" {
		headerText = fmt.Sprintf("%s — %s %s Urgency", title, urgencyEmoji(urgency), urgency)
	}
	// Slack header max is 150 chars
	if len(headerText) > 150 {
		headerText = headerText[:147] + "..."
	}

	blocks := []slackBlock{
		{Type: "header", Text: &slackText{Type: "plain_text", Text: headerText}},
	}

	// Urgency reason (only for Critical/High)
	upperUrgency := strings.ToUpper(strings.TrimSpace(urgency))
	if (upperUrgency == "CRITICAL" || upperUrgency == "HIGH") && urgencyReason != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("⚠️ %s", urgencyReason)},
		})
	}

	// Changelog summary
	summary := report.ChangelogSummary
	if summary == "" {
		summary = report.Summary
	}
	if summary != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: summary},
		})
	}

	// First download command only
	if len(report.DownloadCommands) > 0 {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("`%s`", report.DownloadCommands[0])},
		})
	}

	// Footer context with source info and links
	var footerParts []string
	if msg.Provider != "" && msg.Repository != "" {
		footerParts = append(footerParts, fmt.Sprintf("%s · %s", ProviderLabel(msg.Provider), msg.Repository))
	}
	if msg.SourceURL != "" {
		footerParts = append(footerParts, fmt.Sprintf("<%s|View on %s>", msg.SourceURL, ProviderLabel(msg.Provider)))
	}
	if msg.ReleaseURL != "" {
		footerParts = append(footerParts, fmt.Sprintf("<%s|View in Changelogue>", msg.ReleaseURL))
	}
	if len(footerParts) > 0 {
		blocks = append(blocks, slackBlock{
			Type: "context",
			Elements: []slackText{
				{Type: "mrkdwn", Text: strings.Join(footerParts, "  |  ")},
			},
		})
	}

	return blocks
}
```

Also update the `Send` method detection (line 189) — change `report.Subject != ""` to also accept reports identified by `urgency` for forward compat:

The existing detection `report.Subject != ""` is fine — no change needed since both old and new reports have `subject`.

**Step 4: Run tests**

Run: `go test ./internal/routing/ -v -run TestSlackSender`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/routing/slack.go internal/routing/slack_test.go
git commit -m "refactor(slack): compact two-tier notification format"
```

---

### Task 5: Rewrite Discord buildSemanticEmbed to compact format

**Files:**
- Modify: `internal/routing/discord.go:50-64` (discordRiskColor → discordUrgencyColor)
- Modify: `internal/routing/discord.go:66-150` (buildSemanticEmbed)
- Modify: `internal/routing/discord_test.go`

**Step 1: Add a Discord semantic report test**

Add to `discord_test.go`:

```go
func TestDiscordSender_SemanticReport(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := &DiscordSender{Client: srv.Client()}
	ch := &models.NotificationChannel{
		Type:   "discord",
		Config: json.RawMessage(`{"webhook_url": "` + srv.URL + `"}`),
	}

	reportJSON := `{
		"subject": "Ready to Deploy: go-ethereum v1.16.4",
		"urgency": "Critical",
		"urgency_reason": "Hard fork — deploy before block 18M",
		"changelog_summary": "Consensus-critical update",
		"download_commands": ["docker pull ethereum/client-go:v1.16.4"],
		"availability": "GA"
	}`

	msg := Notification{
		Title:       "go-ethereum v1.16.4",
		Body:        reportJSON,
		Version:     "v1.16.4",
		ProjectName: "go-ethereum",
		Provider:    "github",
		Repository:  "ethereum/go-ethereum",
		SourceURL:   "https://github.com/ethereum/go-ethereum/releases/tag/v1.16.4",
		ReleaseURL:  "https://changelogue.example.com/sr/1",
	}

	err := sender.Send(context.Background(), ch, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload discordPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}

	if len(payload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(payload.Embeds))
	}

	embed := payload.Embeds[0]

	// Should contain urgency reason for Critical
	if !strings.Contains(embed.Description, "Hard fork") {
		t.Fatal("critical urgency should include urgency_reason in description")
	}

	// Should contain changelog
	if !strings.Contains(embed.Description, "Consensus-critical update") {
		t.Fatal("should include changelog summary")
	}

	// Should NOT have inline fields (compact format removes them)
	if len(embed.Fields) > 0 {
		t.Fatalf("compact format should not have embed fields, got %d", len(embed.Fields))
	}

	// Color should be CRITICAL near-black
	if embed.Color != 0x111113 {
		t.Fatalf("expected critical color 0x111113, got 0x%06X", embed.Color)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/routing/ -v -run TestDiscordSender_SemanticReport`
Expected: FAIL (old format has embed fields)

**Step 3: Rename discordRiskColor to discordUrgencyColor and rewrite buildSemanticEmbed**

Rename `discordRiskColor` to `discordUrgencyColor` (same logic, just rename):

```go
// discordUrgencyColor returns a Discord embed color based on urgency level.
func discordUrgencyColor(level string) int {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "CRITICAL":
		return 0x111113 // near-black
	case "HIGH":
		return 0xDC2626 // red
	case "MEDIUM":
		return 0xD97706 // amber
	case "LOW":
		return 0x16A34A // green
	default:
		return 0x5865F2 // Discord blurple
	}
}
```

Rewrite `buildSemanticEmbed`:

```go
// buildSemanticEmbed builds a compact Discord embed from a SemanticReport.
func buildSemanticEmbed(title string, version string, report *models.SemanticReport, msg Notification) discordEmbed {
	// Resolve urgency from new field or backward-compat risk_level
	urgency := report.Urgency
	if urgency == "" {
		urgency = report.RiskLevel
	}
	urgencyReason := report.UrgencyReason
	if urgencyReason == "" {
		urgencyReason = report.RiskReason
	}

	var descParts []string

	// Urgency badge
	if urgency != "" {
		descParts = append(descParts, fmt.Sprintf("%s **%s Urgency**", urgencyEmoji(urgency), urgency))
	}

	// Urgency reason (only for Critical/High)
	upperUrgency := strings.ToUpper(strings.TrimSpace(urgency))
	if (upperUrgency == "CRITICAL" || upperUrgency == "HIGH") && urgencyReason != "" {
		descParts = append(descParts, urgencyReason)
	}

	// Changelog summary
	summary := report.ChangelogSummary
	if summary == "" {
		summary = report.Summary
	}
	if summary != "" {
		descParts = append(descParts, summary)
	}

	// First download command
	if len(report.DownloadCommands) > 0 {
		descParts = append(descParts, fmt.Sprintf("`%s`", report.DownloadCommands[0]))
	}

	// Links
	var linkParts []string
	if msg.SourceURL != "" {
		linkParts = append(linkParts, fmt.Sprintf("[View on %s](%s)", ProviderLabel(msg.Provider), msg.SourceURL))
	}
	if msg.ReleaseURL != "" {
		linkParts = append(linkParts, fmt.Sprintf("[View in Changelogue](%s)", msg.ReleaseURL))
	}
	if len(linkParts) > 0 {
		descParts = append(descParts, strings.Join(linkParts, "  •  "))
	}

	description := strings.Join(descParts, "\n\n")
	if len(description) > discordEmbedDescriptionLimit {
		description = description[:discordEmbedDescriptionLimit-3] + "..."
	}

	// Footer with source info
	var footerText string
	if msg.Provider != "" && msg.Repository != "" {
		footerText = fmt.Sprintf("%s · %s", ProviderLabel(msg.Provider), msg.Repository)
	}

	embed := discordEmbed{
		Title:       title,
		Description: description,
		Color:       discordUrgencyColor(urgency),
	}
	if footerText != "" {
		embed.Footer = &discordEmbedFooter{Text: footerText}
	}
	return embed
}
```

**Step 4: Run all Discord tests**

Run: `go test ./internal/routing/ -v -run TestDiscord`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/routing/discord.go internal/routing/discord_test.go
git commit -m "refactor(discord): compact two-tier notification format"
```

---

### Task 6: Update frontend TypeScript types and detail component

**Files:**
- Modify: `web/lib/api/types.ts:117-130`
- Modify: `web/components/semantic-releases/semantic-release-detail.tsx`

**Step 1: Add urgency_reason to SemanticReport TypeScript interface**

In `web/lib/api/types.ts`, update the `SemanticReport` interface:

```typescript
export interface SemanticReport {
  subject?: string;
  urgency?: string;
  urgency_reason?: string;
  status_checks?: string[];
  changelog_summary?: string;
  download_commands?: string[];
  download_links?: string[];
  summary?: string;
  availability?: string;
  adoption?: string;
  recommendation?: string;
  // Backward compat — old reports may still have these
  risk_level?: string;
  risk_reason?: string;
}
```

**Step 2: Update SemanticReleaseDetail component**

In `semantic-release-detail.tsx`, update the risk banner section to prefer urgency fields with fallback:

Change `getRiskColors` parameter usage — replace `riskLevel` derivation (around line 149):

```typescript
const riskLevel = (report?.urgency ?? report?.risk_level)?.toUpperCase();
```

Replace `report.risk_reason` reference (around line 282):

```typescript
{(report.urgency_reason ?? report.risk_reason) && (
  <p ...>
    {report.urgency_reason ?? report.risk_reason}
  </p>
)}
```

Update the urgency display (around line 274-280) — remove the separate urgency label since it's now the primary field:

```typescript
{riskLevel && (
  <span
    className="rounded px-1.5 py-0.5 text-[11px] font-bold uppercase"
    style={{
      backgroundColor: riskColors.border,
      color: "#ffffff",
    }}
  >
    {riskLevel} URGENCY
  </span>
)}
```

Remove the separate `report.urgency` span since urgency is now the primary field shown in the badge.

**Step 3: Verify frontend builds**

Run: `cd web && npx next build` (or `npx tsc --noEmit`)
Expected: No type errors

**Step 4: Commit**

```bash
git add web/lib/api/types.ts web/components/semantic-releases/semantic-release-detail.tsx
git commit -m "refactor(web): use urgency/urgency_reason with backward compat fallback"
```

---

### Task 7: Run full test suite and verify

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: ALL PASS

**Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues

**Step 3: Final commit if any remaining changes**

No commit unless fixups are needed.
