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

Opting in is a single global action — users do not select a channel. The bot only
DMs someone their reply from a thread they actually replied in, so participation in
a channel's thread is what binds a user to that channel. Someone who replies in two
standup channels receives one DM per channel.

### Daily Flow

1. A scheduled standup message is posted in a watched team channel
2. The bot matches it against that channel's thread identifier and finds the most recent previous standup thread in the same channel
3. For each opted-in user who replied in the previous thread, the bot sends a DM containing their previous reply, today's date prompt, and a link to the new thread:

```
*Tuesday, February 24*
:construction: Continue with ticket

*Wednesday, February 25*
What are you up to today?

<https://…|Open today's standup thread>
```

Lines in the previous reply that are bare dates (`Weekday, Month D`) are bolded, idempotently — already-bolded lines are left alone.

4. The user copies, edits, and posts their update in the new thread

## Architecture

### Deployment Model

- Runs on an always-on EC2 instance as a systemd service (`standup-echo.service`)
- Uses Slack Socket Mode (persistent WebSocket) — no cloud hosting, public URLs, or inbound firewall rules required
- Also distributable as a Go binary via Homebrew with `brew services`, or via `install.sh` on Linux

### Multi-Team Model

A single process, Slack app, and token serve every configured channel:

- Config holds a list of channels, each pairing a `channel_id` with its own `thread_identifier`, since teams word their scheduled standup message differently
- The bot builds one standup service per channel and routes an incoming message by its channel ID, matching only that channel's identifier
- Adding a team means inviting the bot to the channel and appending a config entry — no second instance or app

Implications of one shared instance:

- The bot token gains read access to every channel it is invited to, and that content passes through the host and its logs. The instance operator is effectively trusted by every participating team.
- All teams share one uptime and one deploy. A restart or a bad config affects everyone.

### Data Storage

- **Configuration** (`~/.config/standup-echo/config.yml`): Slack tokens plus a list of channels, each with its own thread identifier
- **Subscribers** (`~/.config/standup-echo/standup-echo.db`): SQLite database with opted-in user IDs, global across channels
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
| `channels[].channel_id` | Slack channel where standups are posted | `C0123456789` |
| `channels[].thread_identifier` | Text matched against the standup **message body** | `async standup check-in` |
| `channels[].name` | Optional label for logs and `trigger --channel` | `m2` |

Configs predating multi-channel support use top-level `channel_id` /
`thread_identifier`; these are folded into `channels` on load and cleared on next
save, so no manual migration is needed.

### Thread Identifier Caveat

The identifier is matched against message text, not the sender's display name. A
standup posted by a Slack Workflow named "Daily Standup" whose body reads *"Hey
team, it's time for our daily async standup check-in!"* will not match
`Daily Standup` — the name is not in the text. Choose a phrase from the body.

## Constraints

- The bot must be running when the standup thread is posted to detect it — there is no catch-up scan on start, so a restart spanning the scheduled post means that day is missed (recoverable with `trigger`)
- Only one instance should run at a time to avoid duplicate DMs
- Users must explicitly opt in — no unsolicited DMs
- History scans are bounded to the last 21 days, so a channel with no standup thread in that window reports none found

## Rollout Plan

1. **Phase 1 — Solo testing**: Bot runs on the developer's company laptop with only the developer subscribed. Validates end-to-end flow: thread detection, reply retrieval, DM delivery.
2. **Phase 2 — Team rollout**: Share the bot with the team. Users opt in via DM. Monitor for issues.

## CLI Reference

| Command | Description |
|---------|-------------|
| `standup-echo serve` | Start the bot daemon (watches every configured channel) |
| `standup-echo trigger` | Manually process a thread; `--channel`, `--date`, `--user` |
| `standup-echo configure` | Interactive configuration setup |
| `standup-echo version` | Print version information |

## Future Considerations

- Reply-to-thread: post the update directly as a thread reply instead of DM-only
- Slash command (`/standup`) to fetch previous update on demand
- Per-channel opt-out, for someone in two standup channels who only wants DMs from one. Requires a `(user_id, channel_id)` subscribers table and a `subscribe <channel>` DM command; deferred until someone actually straddles two channels.
- Match the standup post by workflow/bot identity rather than message text, which would survive teams rewording their prompt
