# Session Context

## User Prompts

### Prompt 1

start the local web

### Prompt 2

sure

### Prompt 3

[Request interrupted by user]

### Prompt 4

I need you to reuse releaseguard-pg

### Prompt 5

There's no release for the git repo: https://github.com/FraxFinance/fraxtal-op-reth
but it has tags 
In our current code, do we support only tags?

### Prompt 6

I think we need two settings here:
1. Only release option, and by default it is on. We can optionally turn off that option so that the tag-only would be discovered as well.
2. Now we have the excluded pre-release option. We can, by default, turn it on.

### Prompt 7

Base directory for this skill: /Users/pc/.claude/skills/nextjs-typecheck

# Next.js TypeScript Type Check

Run `npx tsc --noEmit` in a Next.js frontend directory to verify all TypeScript types are correct.

## Usage

Invoke after editing TypeScript/TSX files in a Next.js project.

## Parameters

- `web` (optional): Path to the web/frontend directory (default: `./web`)

## Execution

1. Run the companion script:

```bash
bash /Users/pc/.claude/skills/nextjs-typecheck/scripts/nextjs-typecheck.sh $...

### Prompt 8

commit this changes looks good

### Prompt 9

Base directory for this skill: /Users/pc/.claude/skills/git-ship

# Git Ship

Inspect working tree state, stage changes, commit with a conventional commit message, and optionally push.

## Usage

Invoke when you have completed a code change and want to commit. The skill performs the standard pre-commit inspection sequence: `git status` → `git diff` → `git log` to understand context before staging and committing.

## Parameters

- `` (optional): Include `push` to also push after committing

#...

