# standup-echo

A Slack bot that DMs each user their previous standup reply when a new standup thread appears. Runs locally via Socket Mode — no cloud hosting needed.

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
6. Invite the bot to the standup channel

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
- **Channel ID** — right-click the channel in Slack > Copy link > extract the ID
- **Thread Identifier** — text that appears in every standup post (e.g. "Daily Standup")

Config is saved to `~/.config/standup-echo/config.yml`.

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

## Commands

| Command                  | Description                          |
|--------------------------|--------------------------------------|
| `standup-echo serve`      | Start the bot daemon                 |
| `standup-echo configure`  | Interactive configuration setup      |
| `standup-echo version`    | Print version information            |
