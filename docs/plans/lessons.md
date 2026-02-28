# Lessons Learned

## 2026-02-25

### Implementation plans must include doc updates inline

**Pattern:** When writing implementation plans, always include updates to existing documentation (ARCH.md, DESIGN.md, API design docs) as part of each relevant task — not as a standalone final task. Each task should update the docs that correspond to the code it changes.

**Why:** Deferring doc updates to the end means they get forgotten or become a massive catch-up task. Inline doc updates keep documentation in sync with code changes and make each commit self-contained.

**Rule:** For every task that changes architecture, models, API endpoints, or component design:
- Include the relevant doc files in the task's "Files:" list
- Add a step to update the corresponding doc section
- Include the doc files in the commit

### Notification format must be aligned across all channels

**Pattern:** When adding or changing information displayed in one channel (Slack, Discord, webhook, web UI), check ALL other channels and update them to show the same data fields. Never add rich formatting to one channel without updating the others.

**Why:** Slack had full SemanticReport parsing (risk, urgency, status checks, download links, etc.) while Discord just dumped raw JSON as description text. Users on different channels got wildly different notification quality. The web UI also had its own representation that diverged from both.

**Rule:** All notification representations (Slack, Discord, webhook, web UI) must show the same semantic report fields:
- Subject, Risk Level + Reason, Urgency, Status Checks, Changelog Summary, Adoption, Download Commands, Download Links
- Each channel should use its native formatting (Block Kit for Slack, Embeds for Discord, structured JSON for webhook)
- When adding a new field to the SemanticReport model, update ALL senders and the web detail component
- Test notifications across all configured channels, not just one
