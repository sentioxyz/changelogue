# Session Context

## User Prompts

### Prompt 1

Can we leverage https://github.com/google/adk-web to build our agent?

### Prompt 2

No, actually we can, please refer to https://google.github.io/adk-docs/get-started/go/#run-with-command-line-interface

### Prompt 3

1

### Prompt 4

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/4.3.1/skills/brainstorming

# Brainstorming Ideas Into Designs

## Overview

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design and get user approval.

<HARD-GATE>
Do NOT invoke any implementat...

### Prompt 5

A

### Prompt 6

Look right

### Prompt 7

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/4.3.1/skills/writing-plans

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the engineer has zero context for our codebase and questionable taste. Document everything they need to know: which files to touch for each task, code, testing, docs they might need to check, how to test it. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent commits.

Ass...

### Prompt 8

1

### Prompt 9

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/4.3.1/skills/subagent-driven-development

# Subagent-Driven Development

Execute plan by dispatching fresh subagent per task, with two-stage review after each: spec compliance review first, then code quality review.

**Core principle:** Fresh subagent per task + two-stage review (spec then quality) = high quality, fast iteration

## When to Use

```dot
digraph when_to_use {
    "Have implementation...

### Prompt 10

tionResponse:
id: "call_xNvQFhkOBmqsYQ6EIbcokQGF"
name: "get_releases"
response:
error: "validating root: validating /properties/releases: validating /properties/releases/items: validating /properties/releases/items/properties/raw_data: type: map[changelog:<p>This is a Testnet release. It is also recommended for Devnet. It is not yet recommended for Mainnet Beta.</p> <p><a href="https://github.com/anza-xyz/agave/blob/v3.1/CHANGELOG.md">https://github.com/anza-xyz/agave/blob/v3.1/CHANGELOG.md</a>...

### Prompt 11

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/4.3.1/skills/systematic-debugging

# Systematic Debugging

## Overview

Random fixes waste time and create new bugs. Quick patches mask underlying issues.

**Core principle:** ALWAYS find root cause before attempting fixes. Symptom fixes are failure.

**Violating the letter of this process is violating the spirit of debugging.**

## The Iron Law

```
NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST
...

