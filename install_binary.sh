#!/usr/bin/env bash
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BINARY_PATH=/usr/local/bin/files_watcher

if [ "$(id -u)" -ne 0 ]; then
	printf 'Run this installer as root: sudo %s\n' "$0" >&2
	exit 1
fi

if command -v go >/dev/null 2>&1; then
	BUILD_DIR=$(mktemp -d)
	trap 'rm -f "$BUILD_DIR/files_watcher"; rmdir "$BUILD_DIR"' EXIT

	cd "$SCRIPT_DIR"
	CGO_ENABLED=0 GOOS=linux go build -trimpath -o "$BUILD_DIR/files_watcher" .
	install -D -m 0755 "$BUILD_DIR/files_watcher" "$BINARY_PATH"
elif [ -x "$SCRIPT_DIR/files_watcher" ]; then
	install -D -m 0755 "$SCRIPT_DIR/files_watcher" "$BINARY_PATH"
else
	printf 'Go is unavailable and no packaged binary was found in %s.\n' "$SCRIPT_DIR" >&2
	exit 1
fi

printf 'Installed static Linux binary to %s\n' "$BINARY_PATH"
