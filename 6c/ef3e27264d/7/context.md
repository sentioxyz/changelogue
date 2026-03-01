# Session Context

## User Prompts

### Prompt 1

Implement the following plan:

# Switch GitHub Source from Atom Feed to REST API

## Context

The GitHub source currently uses the Atom feed (`/releases.atom`) which doesn't include the `prerelease` boolean. To support "Exclude Pre-Releases" for GitHub sources, we need to switch to the GitHub REST API (`/repos/{owner}/{repo}/releases`) which provides `prerelease`, `draft`, `body` (markdown changelog), and richer metadata.

The `prerelease` field will be stored in the release's `raw_data` JSONB a...

### Prompt 2

where can I set the prerelease filter in ux

### Prompt 3

I hope we can have these filters settings while adding the sources not editing them later

### Prompt 4

There another place when we create project

### Prompt 5

Let's also include the version filter there

### Prompt 6

cool, then should we remove the version filter in the subscription or we keep it, what's your opinion?

### Prompt 7

cool, let's commit and push

