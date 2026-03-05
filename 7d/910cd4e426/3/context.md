# Session Context

## User Prompts

### Prompt 1

There's a bug that when the release log is empty, the slack notification, show zksync{"prerelease": "false", "release_url": "https://github.com/matter-labs/zksync-era/releases/tag/test-release-aba"}, for dockerhub it shows `external-node{}` please investigate and fix all the related issues

### Prompt 2

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

### Prompt 3

commit and push

