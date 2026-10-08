#!/usr/bin/env bash
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ "$(id -u)" -ne 0 ]; then
	printf 'Run this installer as root: sudo %s\n' "$0" >&2
	exit 1
fi

"$SCRIPT_DIR/install_binary.sh"
"$SCRIPT_DIR/install_service.sh"
