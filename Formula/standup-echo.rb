class StandupEcho < Formula
  desc "Slack bot that DMs your previous standup update when a new thread appears"
  homepage "https://github.com/j4rs/standup-echo"
  url "https://github.com/j4rs/standup-echo/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "PLACEHOLDER"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/j4rs/standup-echo/cmd.version=#{version}
      -X github.com/j4rs/standup-echo/cmd.commit=#{tap.user}
      -X github.com/j4rs/standup-echo/cmd.date=#{time.iso8601}
    ]
    system "go", "build", *std_go_args(ldflags:)
  end

  service do
    run [opt_bin/"standup-echo", "serve"]
    run_type :immediate
    keep_alive true
    log_path var/"log/standup-echo/output.log"
    error_log_path var/"log/standup-echo/error.log"
  end

  def caveats
    <<~EOS
      Before starting the service, configure the bot:
        standup-echo configure

      Then start the service:
        brew services start standup-echo

      Required Slack app scopes:
        Bot: channels:history, groups:history, chat:write, im:write, im:history
        App-level: connections:write

      Event subscriptions:
        message.channels, message.groups, message.im

      Users must DM the bot "subscribe" to opt in.
    EOS
  end

  test do
    assert_match "standup-echo", shell_output("#{bin}/standup-echo version")
  end
end
