#!/bin/bash
set -e

# Source credentials
source ./env.sh

# Find Claude Code command
if [ -n "$CLAUDE_CMD" ]; then
    # User override
    CLAUDE="$CLAUDE_CMD"
elif command -v claude-code &>/dev/null; then
    CLAUDE="claude-code"
elif command -v claude &>/dev/null; then
    CLAUDE="claude"
elif [ -x "$HOME/.claude/local/claude" ]; then
    CLAUDE="$HOME/.claude/local/claude"
elif [ -x "/usr/local/bin/claude" ]; then
    CLAUDE="/usr/local/bin/claude"
elif [ -x "/usr/local/bin/claude-code" ]; then
    CLAUDE="/usr/local/bin/claude-code"
else
    echo "Error: Claude Code not found. Tried:" >&2
    echo "  - claude-code command" >&2
    echo "  - claude command" >&2
    echo "  - ~/.claude/local/claude" >&2
    echo "  - /usr/local/bin/claude" >&2
    echo "Set CLAUDE_CMD=/path/to/claude or install from https://github.com/anthropics/claude-code" >&2
    exit 1
fi

# Run the agent workflow
# Invoke the three subagents in sequence
exec "$CLAUDE" --model haiku "Execute the prompt in @PROMPT.md"
