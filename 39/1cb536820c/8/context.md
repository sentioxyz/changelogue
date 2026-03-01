# Session Context

## User Prompts

### Prompt 1

I have these projects on newreleases.io, please help migrate to our prod, some of them already been added to our product.
{
    "projects": [
        {
            "id": "2p0psc7vc857bep8ce7q0gz938",
            "name": "MystenLabs/sui",
            "provider": "github",
            "url": "https://github.com/MystenLabs/sui",
            "email_notification": "d",
            "slacks": [
                "ww0wgt0d0fcjnehpxsdhjx1r58"
            ],
            "telegram_chats": [],
            "di...

### Prompt 2

[Request interrupted by user for tool use]

### Prompt 3

Yes, continue

### Prompt 4

why some releases' project is shown as guid in the releases page and provider is null?

### Prompt 5

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

