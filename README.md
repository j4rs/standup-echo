# standup-echo

A Slack bot that DMs each user their previous standup reply when a new standup thread appears. Runs via Socket Mode — no public URL or inbound firewall rules needed.

One instance serves any number of teams. Each channel gets its own thread identifier, so teams whose standup message is worded differently can share a single bot and Slack app.

## Slack App Setup

1. Create a new Slack app at https://api.slack.com/apps
2. Enable **Socket Mode** under Settings and generate an App-Level Token with the `connections:write` scope
3. Add the following **Bot Token Scopes** under OAuth & Permissions:
   - `channels:history` — read messages in public channels
   - `groups:history` — read messages in private channels
   - `chat:write` — send messages as the bot
   - `im:write` — send direct messages
   - `im:history` — read DM messages (for subscribe/unsubscribe commands)
4. Subscribe to these events under Event Subscriptions:
   - `message.channels`
   - `message.groups`
   - `message.im`
5. Install the app to your workspace
6. Invite the bot to each standup channel (`/invite @your-bot`) — including private ones

## Installation

### Linux (one-liner)

```bash
curl -fsSL https://raw.githubusercontent.com/j4rs/standup-echo/main/install.sh | sudo bash
```

Installs Go and Git if needed, builds the binary to `/usr/local/bin`, and creates a systemd service.

### Homebrew (macOS)

```bash
brew tap j4rs/tools
brew install standup-echo
```

### From Source

```bash
git clone https://github.com/j4rs/standup-echo.git
cd standup-echo
make build
```

## Configuration

```bash
standup-echo configure
```

You'll be prompted for:
- **Slack Bot Token** (`xoxb-...`) — from OAuth & Permissions
- **Slack App Token** (`xapp-...`) — the App-Level Token with `connections:write`
- Then, for each channel:
  - **Channel ID** — right-click the channel in Slack > Copy link > extract the ID
  - **Thread Identifier** — text from the standup message **body** (see below)
  - **Name** — optional short label used in logs and `trigger --channel`

Config is saved to `~/.config/standup-echo/config.yml`:

```yaml
slack_bot_token: xoxb-...
slack_app_token: xapp-...
channels:
  - name: m1
    channel_id: C0123456789
    thread_identifier: Daily Standup
  - name: m2
    channel_id: C9876543210
    thread_identifier: async standup check-in
```

Pre-multi-channel configs with top-level `channel_id` / `thread_identifier` still
load — they're folded into `channels` automatically.

### Picking a thread identifier

The identifier is matched against the **message text**, not the sender's name. This
matters for standups posted by a Slack Workflow: a workflow named "Daily Standup"
whose message reads *"Hey team, it's time for our daily async standup check-in!"*
will **never** match `Daily Standup` — that string is only the display name. Pick a
distinctive phrase from the body itself, e.g. `async standup check-in`.

To check what the bot actually sees, run `standup-echo trigger --channel <name>`
after inviting the bot; it logs `found standup thread` on a match and
`no standup thread found` otherwise.

## Usage

### Run directly

```bash
standup-echo serve
```

### Run as a service (macOS)

```bash
brew services start standup-echo
```

The bot starts on login and watches for new standup threads. When one appears, it finds the previous thread, collects each user's reply, and DMs subscribed users their update as a ready-to-edit template.

### Opting in

The bot is **opt-in only**. Each user must DM the bot to subscribe:

- Send `subscribe` to start receiving standup reminders
- Send `unsubscribe` to stop

Subscriber data is stored locally at `~/.config/standup-echo/standup-echo.db`.

Subscribing is a single global opt-in — there's no need to name a channel. A DM is
only ever sent to someone who replied in that channel's previous standup thread, so
the reply itself establishes which team you're on. Anyone in two standup channels
gets one DM per channel.

## Commands

| Command                  | Description                          |
|--------------------------|--------------------------------------|
| `standup-echo serve`      | Start the bot daemon                 |
| `standup-echo trigger`    | Manually re-send DMs for a thread    |
| `standup-echo configure`  | Interactive configuration setup      |
| `standup-echo version`    | Print version information            |

### trigger

Useful for verifying a new channel's identifier or re-sending a missed day:

```bash
standup-echo trigger --channel m2                   # most recent m2 standup thread
standup-echo trigger --channel m2 --date 2026-08-13
standup-echo trigger --channel m2 --user U0123456   # limit to one person
```

`--channel` takes a channel ID or the configured `name`, and is required when more
than one channel is configured. Note that `trigger` echoes replies from the thread
it finds back to their authors, whereas `serve` echoes the *previous* thread into
the newly posted one.
