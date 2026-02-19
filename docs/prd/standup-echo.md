# Standup Echo — Product Requirements Document

## Problem

Daily standups are done by replying to a scheduled Slack thread. To write a new update, team members need to reference their previous one, which means manually scrolling through old threads to find what they wrote last time. This is tedious and slows down the standup process.

## Solution

Standup Echo is a Slack bot that automatically DMs each opted-in user their previous standup reply when a new standup thread appears. The message serves as a ready-to-edit template so users can quickly update and post without hunting through old threads.

## User Experience

### Opting In

Users DM the bot to manage their subscription:

- Send `subscribe` to start receiving standup reminders
- Send `unsubscribe` to stop at any time
- Any other message returns a help prompt with available commands

### Daily Flow

1. A scheduled standup message is posted in the team channel (e.g. "Daily Standup — February 19")
2. The bot detects the new thread and finds the most recent previous standup thread
3. For each opted-in user who replied in the previous thread, the bot sends a DM:

```
Your previous standup update (from Monday, February 18):
---
(a) Completed: ...
(b) Working on: ...
(c) Blockers: ...
---
Copy and update for today's standup!
```

4. The user copies, edits, and posts their update in the new thread

## Architecture

### Deployment Model

- Runs locally on a single company laptop as a background service
- Uses Slack Socket Mode (persistent WebSocket) — no cloud hosting, public URLs, or inbound firewall rules required
- Distributed as a Go binary via Homebrew with `brew services` for launch-on-login

### Data Storage

- **Configuration** (`~/.config/standup-echo/config.yml`): Slack tokens, channel ID, thread identifier
- **Subscribers** (`~/.config/standup-echo/standup-echo.db`): SQLite database with opted-in user IDs
- No message content is stored — previous replies are fetched from Slack on demand

### Slack App Scopes

| Scope | Purpose |
|-------|---------|
| `channels:history` | Read messages in the standup channel (public) |
| `groups:history` | Read messages in the standup channel (private) |
| `chat:write` | Send messages as the bot |
| `im:write` | Send direct messages to subscribers |
| `im:history` | Receive subscribe/unsubscribe commands via DM |
| `connections:write` | App-level scope for Socket Mode |

### Event Subscriptions

- `message.channels` — detect new standup threads in public channels
- `message.groups` — detect new standup threads in private channels
- `message.im` — receive DM commands from users

## Configuration

| Field | Description | Example |
|-------|-------------|---------|
| `slack_bot_token` | Bot OAuth token | `xoxb-...` |
| `slack_app_token` | App-level token with `connections:write` | `xapp-...` |
| `channel_id` | Slack channel where standups are posted | `C0123456789` |
| `thread_identifier` | Text pattern to match standup thread messages | `Daily Standup` |

## Constraints

- The bot must be running (laptop on and awake) when the standup thread is posted to detect it
- Only one instance should run at a time to avoid duplicate DMs
- Users must explicitly opt in — no unsolicited DMs

## Rollout Plan

1. **Phase 1 — Solo testing**: Bot runs on the developer's company laptop with only the developer subscribed. Validates end-to-end flow: thread detection, reply retrieval, DM delivery.
2. **Phase 2 — Team rollout**: Share the bot with the team. Users opt in via DM. Monitor for issues.

## CLI Reference

| Command | Description |
|---------|-------------|
| `standup-echo serve` | Start the bot daemon |
| `standup-echo configure` | Interactive configuration setup |
| `standup-echo version` | Print version information |

## Future Considerations

- Deploy to an always-on machine for reliability
- Reply-to-thread: post the update directly as a thread reply instead of DM-only
- Slash command (`/standup`) to fetch previous update on demand
