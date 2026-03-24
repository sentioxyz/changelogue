# Session Context

## User Prompts

### Prompt 1

I want the release notification can be optionally posponed until semantic analysis found that all the sources of this specific version are avaible (or the critical source). For exmaple, a 3rd party builds the docker image and publish it in its own dockerhub, and it could be several months latency behind the github release. How can we achieve this?

### Prompt 2

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/5.0.5/skills/brainstorming

# Brainstorming Ideas Into Designs

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design and get user approval.

<HARD-GATE>
Do NOT invoke any implementation skill, wr...

### Prompt 3

Approach 1

### Prompt 4

I'm thinking if this release gate should be applied to the semantic report generation as well

### Prompt 5

Looks good, there's another requirment that in the most cases, one project one source, one semantic release report per source release, so we want to get the notification only when the semantic report generated for the specific version, so the notification can be merged on one (now two sperated). Like source v1 released (urgency high) ...

### Prompt 6

there should be no release gate for single source subscription right?

### Prompt 7

Yes

### Prompt 8

For the Single-source project, users can still choose source release only notification right?

### Prompt 9

Yes

### Prompt 10

Yes

### Prompt 11

Yes

### Prompt 12

Yes

### Prompt 13

Yes

### Prompt 14

continue

### Prompt 15

Base directory for this skill: /Users/pc/.claude/plugins/cache/superpowers-marketplace/superpowers/5.0.5/skills/writing-plans

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the engineer has zero context for our codebase and questionable taste. Document everything they need to know: which files to touch for each task, code, testing, docs they might need to check, how to test it. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent commits.

Ass...

### Prompt 16

continue

### Prompt 17

continue

