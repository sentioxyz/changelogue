# Session Context

## User Prompts

### Prompt 1

Please apply the similar filter ribbon of releases page to the subscriptions page, include the release chanel and the release type

### Prompt 2

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

### Prompt 3

Also please help me merge the select all checkbox and the ribbon shows total subscriptions with the title of the channels

### Prompt 4

Then the filter ribbon shouldn't have the select all checkbox  the select all check box should be on the sheet's header and once click it , the header will change to include the delete button

### Prompt 5

The `Delete selected` button is too close to the "1 of 5 selected"

### Prompt 6

The style still looks not very good, can you design the header better?

### Prompt 7

much better, push the changes

### Prompt 8

Base directory for this skill: /Users/pc/.claude/skills/git-ship

# Git Ship

Inspect working tree state, stage changes, commit with a conventional commit message, and optionally push.

## Usage

Invoke when you have completed a code change and want to commit. The skill performs the standard pre-commit inspection sequence: `git status` → `git diff` → `git log` to understand context before staging and committing.

## Parameters

- `` (optional): Include `push` to also push after committing

#...

