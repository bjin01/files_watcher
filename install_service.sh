#!/usr/bin/env bash
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
CONFIG_PATH=/etc/files_watcher/config.yaml
UNIT_PATH=/etc/systemd/system/files_watcher.service
BINARY_PATH=/usr/local/bin/files_watcher

if [ "$(id -u)" -ne 0 ]; then
	printf 'Run this installer as root: sudo %s\n' "$0" >&2
	exit 1
fi

if [ ! -x "$BINARY_PATH" ]; then
	printf 'Watcher binary not found or not executable: %s\n' "$BINARY_PATH" >&2
	printf 'Run install_binary.sh first.\n' >&2
	exit 1
fi

if [ ! -f "$CONFIG_PATH" ]; then
	printf 'Configuration file not found: %s\n' "$CONFIG_PATH" >&2
	printf 'Create it with directories and log_file settings before installing the service.\n' >&2
	exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
	printf 'systemctl is required to install and start the service.\n' >&2
	exit 1
fi

install -D -m 0644 "$SCRIPT_DIR/files_watcher.service" "$UNIT_PATH"
systemctl daemon-reload
systemctl enable --now files_watcher.service
printf 'Installed and started files_watcher.service\n'
