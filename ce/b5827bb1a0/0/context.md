# Session Context

## User Prompts

### Prompt 1

The current agent run read all the releases from the project instead of focus the latest one, we should let the agent focus on the specific version, let's have a placeholder in its prompt, and pass a version, then agent should cross check different sources and contexts only related to that release version and gives a report. The report should contain the following information: Subject: :rocket: Ready to Deploy: Geth v1.10.15 (Critical Update) Body:

Status: :white_check_mark: Docker Image Verifi...

### Prompt 2

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/4.3.1/skills/brainstorming

# Brainstorming Ideas Into Designs

## Overview

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design and get user approval.

<HARD-GATE>
Do NOT invoke any implementat...

### Prompt 3

Binary/image availablity can be checked from its sources, web search is only needed when we need additional contexts

### Prompt 4

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/4.3.1/skills/writing-plans

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the engineer has zero context for our codebase and questionable taste. Document everything they need to know: which files to touch for each task, code, testing, docs they might need to check, how to test it. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent commits.

Ass...

