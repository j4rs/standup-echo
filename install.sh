#!/usr/bin/env bash
set -euo pipefail

REPO="github.com/j4rs/standup-echo"
INSTALL_DIR="/usr/local/bin"
SERVICE_USER="${SUDO_USER:-$USER}"
SERVICE_HOME=$(eval echo "~$SERVICE_USER")

echo "==> Installing standup-echo"

# Detect package manager and install Go + Git if needed.
install_deps() {
    if command -v go &>/dev/null && command -v git &>/dev/null; then
        echo "==> Go and Git already installed"
        return
    fi

    echo "==> Installing dependencies..."
    if command -v apt-get &>/dev/null; then
        apt-get update -qq && apt-get install -y -qq golang git
    elif command -v yum &>/dev/null; then
        yum install -y golang git
    elif command -v dnf &>/dev/null; then
        dnf install -y golang git
    else
        echo "Error: unsupported package manager. Install Go and Git manually."
        exit 1
    fi
}

# Build and install the binary.
install_binary() {
    echo "==> Building standup-echo..."
    TMPDIR=$(mktemp -d)
    git clone --depth 1 "https://${REPO}.git" "$TMPDIR/standup-echo"
    cd "$TMPDIR/standup-echo"
    make build
    cp bin/standup-echo "$INSTALL_DIR/standup-echo"
    chmod +x "$INSTALL_DIR/standup-echo"
    rm -rf "$TMPDIR"
    echo "==> Installed to $INSTALL_DIR/standup-echo"
}

# Create systemd service.
install_service() {
    if [ ! -d /etc/systemd/system ]; then
        echo "==> Skipping systemd setup (not available)"
        return
    fi

    echo "==> Creating systemd service..."
    cat > /etc/systemd/system/standup-echo.service <<EOF
[Unit]
Description=Standup Echo Slack Bot
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=$INSTALL_DIR/standup-echo serve
Restart=always
RestartSec=5
User=$SERVICE_USER
Environment=HOME=$SERVICE_HOME

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable standup-echo
    echo "==> Service installed (not started — configure first)"
}

# Require root for system-wide install.
if [ "$(id -u)" -ne 0 ]; then
    echo "Error: run with sudo"
    echo "  curl -fsSL https://raw.githubusercontent.com/j4rs/standup-echo/main/install.sh | sudo bash"
    exit 1
fi

install_deps
install_binary
install_service

echo ""
echo "Done! Next steps:"
echo "  1. standup-echo configure"
echo "  2. sudo systemctl start standup-echo"
echo "  3. DM the bot 'subscribe' in Slack"
