#!/bin/bash
# Load Bluesky credentials from macOS Keychain
#
# Setup (one-time):
#   security add-generic-password -s "bsky-agent" -a "handle" -w "your.handle.bsky.social"
#   security add-generic-password -s "bsky-agent" -a "password" -w "xxxx-xxxx-xxxx-xxxx"

export BSKY_HANDLE=$(security find-generic-password -s "bsky-agent" -a "handle" -w 2>/dev/null)
export BSKY_PASSWORD=$(security find-generic-password -s "bsky-agent" -a "password" -w 2>/dev/null)

if [ -z "$BSKY_HANDLE" ] || [ -z "$BSKY_PASSWORD" ]; then
    echo "Error: Bluesky credentials not found in keychain" >&2
    echo "Please set up credentials using:" >&2
    echo '  security add-generic-password -s "bsky-agent" -a "handle" -w "your.handle.bsky.social"' >&2
    echo '  security add-generic-password -s "bsky-agent" -a "password" -w "your-app-password"' >&2
    exit 1
fi
