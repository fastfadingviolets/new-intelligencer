# Bluesky Daily Digest Agent

A Claude Code agent that summarizes your Bluesky feed and produces a daily report.

## What This Does

This agent fetches your Bluesky timeline from the last 24 hours and provides a concise summary including post counts, top contributors, and interesting patterns.

## Prerequisites

- Go 1.24+
- [Claude Code](https://github.com/anthropics/claude-code)
- Bluesky account with an app password

## Setup

### 1. Generate a Bluesky App Password

1. Open the Bluesky app
2. Go to **Settings � Privacy and Security � App Passwords**
3. Click **"Add App Password"**
4. Name it "Daily Digest Agent"
5. Save the generated password (format: `xxxx-xxxx-xxxx-xxxx`)

### 2. Store Credentials in macOS Keychain

```bash
security add-generic-password -s "bsky-agent" -a "handle" -w "your.handle.bsky.social"
security add-generic-password -s "bsky-agent" -a "password" -w "xxxx-xxxx-xxxx-xxxx"
```

The first time you run the agent, macOS will prompt you to allow access. Click **"Always Allow"** to avoid future prompts.

### 3. Build the Tool

```bash
make
```

This compiles the `bsky` binary to `./bin/bsky`.

## Usage

### Quick Start

```bash
./run.sh
```

This runs the agent with the default prompt: "build my report for today".

### Custom Prompt

```bash
./run.sh "analyze the last 12 hours"
```

### Direct Usage with Claude Code

```bash
source env.sh
claude "Use bsky-digest subagent: your prompt here"
```

## How It Works

1. `env.sh` loads your Bluesky credentials from macOS Keychain
2. Claude Code invokes the `bsky-digest` persistent subagent (configured in `.claude/agents/bsky-digest.md`)
3. The subagent runs `./bin/bsky fetch-feed` to fetch your timeline
4. Claude analyzes the JSON output and generates a summary
5. Results are presented in a concise format

## Subagent System

This project uses Claude Code's persistent subagent system. The `bsky-digest` subagent is configured with:
- **Tools**: Bash (for running the feed fetcher)
- **Model**: Haiku (fast and efficient for digest generation)
- **System Prompt**: Detailed instructions for analyzing feed data and formatting output

The subagent configuration is stored in `.claude/agents/bsky-digest.md` and is automatically available to Claude Code.

## Security

- Credentials are stored in macOS Keychain, not in code
- App passwords are scoped and revokable
- The agent never sees your main Bluesky password
