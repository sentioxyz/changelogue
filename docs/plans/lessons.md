# Lessons Learned

## 2026-02-25

### Implementation plans must include doc updates inline

**Pattern:** When writing implementation plans, always include updates to existing documentation (ARCH.md, DESIGN.md, API design docs) as part of each relevant task — not as a standalone final task. Each task should update the docs that correspond to the code it changes.

**Why:** Deferring doc updates to the end means they get forgotten or become a massive catch-up task. Inline doc updates keep documentation in sync with code changes and make each commit self-contained.

**Rule:** For every task that changes architecture, models, API endpoints, or component design:
- Include the relevant doc files in the task's "Files:" list
- Add a step to update the corresponding doc section
- Include the doc files in the commit
