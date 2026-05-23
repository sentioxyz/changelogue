# Session Context

## User Prompts

### Prompt 1

change the model to opus

### Prompt 2

# Update Config Skill

Modify Claude Code configuration by updating settings.json files.

## When Hooks Are Required (Not Memory)

If the user wants something to happen automatically in response to an EVENT, they need a **hook** configured in settings.json. Memory/preferences cannot trigger automated actions.

**These require hooks:**
- "Before compacting, ask me what to preserve" → PreCompact hook
- "After writing files, run prettier" → PostToolUse hook with Write|Edit matcher
- "When I run...

### Prompt 3

Summarize this conversation in max 5 words for a topic title. Reply ONLY with the title, nothing else.

### Prompt 4

list my latest commits

### Prompt 5

In the project detailed page, for sources, please help me merge provider and repository header into one, and use an icon+repo url, reuse stype in the projects page

### Prompt 6

Base directory for this skill: /Users/pc/.claude/skills/nextjs-typecheck

# Next.js TypeScript Type Check

Run `npx tsc --noEmit` in a Next.js frontend directory to verify all TypeScript types are correct.

## Usage

Invoke after editing TypeScript/TSX files in a Next.js project.

## Parameters

- `` (optional): Path to the web/frontend directory (default: `./web`)

## Execution

1. Run the companion script:

```bash
bash /Users/pc/.claude/skills/nextjs-typecheck/scripts/nextjs-typecheck.sh ${AR...

### Prompt 7

create a pull request

### Prompt 8

Base directory for this skill: /Users/pc/.claude/skills/git-ship

# Git Ship

Inspect working tree state, stage changes, commit with a conventional commit message, and optionally push.

## Usage

Invoke when you have completed a code change and want to commit. The skill performs the standard pre-commit inspection sequence: `git status` → `git diff` → `git log` to understand context before staging and committing.

## Parameters

- `` (optional): Include `push` to also push after committing

#...

