# Session Context

## User Prompts

### Prompt 1

Implement the following plan:

# Replace "Needs Attention" Card with Release Trend Chart

## Context

The "Needs Attention" stat card counts semantic releases with critical/high urgency, but has no mechanism to clear/acknowledge items — the number only grows over time. We're replacing it with a release trend bar chart that shows the velocity of releases and AI reports over time, with switchable daily/weekly/monthly granularity.

## Approach

- New backend endpoint `GET /api/v1/stats/trend?gran...

### Prompt 2

Change the layout to Project tracked and releases this week at the top. Release trend to bottom left (default 7 days) recent activity to bottom right

### Prompt 3

No it still shows the last 30 days

### Prompt 4

This is not flexible to let the backend code to control the interval, this should be a paramter from the api and ux can have the option to change it

