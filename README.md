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
max_missed_standups: 2
reminder_after: 4h
channels:
  - name: m1
    channel_id: C0123456789
    thread_identifier: Daily Standup
  - name: m2
    channel_id: C9876543210
    thread_identifier: async standup check-in
```

Pre-multi-channel configs with top-level `channel_id` / `thread_identifier` still
load — they're folded into `channels` automatically. `max_missed_standups` and
`reminder_after` are both optional; see [Missed standups](#missed-standups) and
[Mid-day reminders](#mid-day-reminders).

### Picking a thread identifier

The identifier is matched against the **message text**, not the sender's name. This
matters for standups posted by a Slack Workflow: a workflow named "Daily Standup"
whose message reads *"Hey team, it's time for our daily async standup check-in!"*
will **never** match `Daily Standup` — that string is only the display name. Pick a
distinctive phrase from the body itself, e.g. `async standup check-in`.

To check what the bot actually sees, run `standup-echo trigger --channel <name>`
after inviting the bot; it logs `found standup thread` on a match and
`no standup thread found` otherwise.

### Missed standups

The DM is what prompts you to write the next update, so delivering it only to
people who replied in the *immediately* preceding thread makes one missed day
self-perpetuating: skip Wednesday, get no nudge Thursday, skip Thursday too.

`max_missed_standups` is how many standups in a row you can miss and still be
nudged. The bot scans back that many threads plus one, and DMs anyone who replied
in any of them, echoing their most recent update. At the default of `2`:

| Last replied      | Nudged today? |
|-------------------|---------------|
| yesterday         | yes           |
| two standups ago  | yes           |
| three standups ago| yes           |
| four or more ago  | no            |

So a forgotten day recovers on its own, while a real absence costs two more DMs
per channel and then goes quiet — and it resumes automatically the next time you
reply in a thread. Set it to `0` to nudge only people who replied in the previous
thread. Note the unit is *standup threads*, not calendar days, so a weekend does
not consume the allowance.

### Mid-day reminders

The morning DM hands you a template; the mid-day one is the actual nag. If you are
on the roster for a channel and haven't posted in today's thread by then, the bot
DMs you a short nudge with a link to it.

`reminder_after` is a Go duration measured **from the moment the standup thread is
posted**, not a wall-clock time. With a standup that posts at 9:15am and the default
`4h`, the reminder lands around 1:15pm. Anchoring to the thread means there is no
timezone to configure, nothing to break at a DST boundary, and the reminder follows
the standup automatically if its scheduled time ever moves. Set `reminder_after: 0`
to turn reminders off.

The roster is stricter than the morning nudge: it is only the people who posted in
the **immediately preceding** thread, ignoring `max_missed_standups`. The morning
grace window exists to recover a loop that went silent, whereas a reminder is a nag
and should decay fast — someone away for a week drops off the roster after one
standup instead of being pinged for days.

Consequences worth knowing:

- **No standup thread today, no reminder.** Weekends, holidays, and a workflow that
  failed to post all need no special handling.
- **One reminder per thread, ever.** Sends are recorded in a `reminders` table, so
  restarting or redeploying mid-afternoon cannot nag anyone twice.
- **A restart is recovered.** On startup the bot scans for today's thread and re-arms
  the timer, unless the window has already passed — a nag hours late is worse than
  none.
- **Reminders are not separately opt-out-able.** `unsubscribe` turns off both the
  echo and the reminder. Splitting them needs the `(user_id, channel_id)` primary key
  that per-channel opt-out was already deferred for.

## Usage

### Run directly

```bash
standup-echo serve
```

### Run as a service (macOS)

```bash
brew services start standup-echo
```

The bot starts on login and watches for new standup threads. When one appears, it scans the recent preceding threads, collects each user's most recent reply, and DMs subscribed users their update as a ready-to-edit template.

### Opting in

The bot is **opt-in only**. Each user must DM the bot to subscribe:

- Send `subscribe` to start receiving standup reminders
- Send `unsubscribe` to stop

Subscriber data is stored locally at `~/.config/standup-echo/standup-echo.db`.

Subscribing is a single global opt-in — there's no need to name a channel. A DM is
only ever sent to someone who replied in one of that channel's recent standup
threads, so the reply itself establishes which team you're on. Anyone in two
standup channels gets one DM per channel.

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
standup-echo trigger --channel m2                   # target the newest m2 thread
standup-echo trigger --channel m2 --date 2026-08-13
standup-echo trigger --channel m2 --user U0123456   # limit delivery to one person
standup-echo trigger --channel m2 --reminder        # rehearse the mid-day reminder
```

`--channel` takes a channel ID or the configured `name`, and is required when more
than one channel is configured.

`trigger` runs the same path as `serve`: it locates the target thread, then echoes
the **preceding** thread's replies into it. A trigger run is therefore a faithful
rehearsal of the live flow, not an approximation of it.

To check a channel's configuration without DMing anyone, pass a `--user` that has no
reply in the thread — the run stops before delivery, having already proven channel
access, identifier matching, and reply collection:

```bash
standup-echo trigger --channel m2 --user UDRYRUN00000
```

`--reminder` sends the mid-day nudge instead of the echo, without waiting for the
timer and without consulting the sent-reminder table, so it can be run repeatedly.
Pair it with `--user` to keep a test to yourself:

```bash
standup-echo trigger --channel m1 --reminder --user U0123456
```

Every run logs a summary line — `finished sending DMs sent=N skipped_not_subscribed=N
failed=N`, or `finished sending reminders ...` — so a run that delivers nothing
explains itself rather than looking broken. The most common cause of `sent=0` is a
recipient who hasn't sent `subscribe`.
