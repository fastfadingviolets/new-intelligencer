# Bluesky Daily Digest Agent

A multi-agent system that creates newspaper-style daily digests from your Bluesky timeline.

## What This Does

This project uses Claude Code agents to transform your Bluesky feed into a curated daily digest:

1. **Fetches** posts from your timeline (last 24 hours)
2. **Categorizes** posts into newspaper sections (tech, sports, music, politics, etc.)
3. **Consolidates** related posts into story groups
4. **Selects** top stories for the front page
5. **Generates** headlines and priority rankings
6. **Compiles** everything into markdown and HTML digests

## Prerequisites

- Go 1.24+
- [Claude Code](https://github.com/anthropics/claude-code)
- Bluesky account with an app password

## Setup

### 1. Generate a Bluesky App Password

1. Open the Bluesky app
2. Go to **Settings → Privacy and Security → App Passwords**
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
make build
```

This compiles the `digest` binary to `./bin/digest`.

## Usage

### Quick Start

```bash
./run.sh
```

This runs the full digest workflow with Claude Code agents.

### Key Commands

```bash
# Initialize a new workspace for today
./bin/digest init

# Fetch posts from your timeline
./bin/digest fetch

# Check workflow progress
./bin/digest status

# Compile the final digest
./bin/digest compile
```

### Using Claude Code Agents

```bash
source env.sh
claude "Use bsky-section-categorizer to process posts"
```

## Workflow Pipeline

The digest creation follows a 6-stage pipeline:

```
FETCH → CATEGORIZE → CONSOLIDATE → FRONT PAGE → HEADLINES → COMPILE
```

| Stage | Agent | Description |
|-------|-------|-------------|
| Fetch | CLI | Pull posts from Bluesky API |
| Categorize | `bsky-section-categorizer` | Sort posts into sections |
| Consolidate | `bsky-consolidator` | Group related posts into stories |
| Front Page | `bsky-front-page-selector` | Pick 4-6 top stories |
| Headlines | `bsky-headline-editor` | Set headlines and priorities |
| Compile | CLI | Generate markdown and HTML |

## Agent Architecture

Four specialized Claude Code agents orchestrate the workflow:

| Agent | Role |
|-------|------|
| **bsky-section-categorizer** | Evaluates posts and assigns them to newspaper sections |
| **bsky-consolidator** | Groups posts about the same story/event together |
| **bsky-front-page-selector** | Curates the most important stories for the front page |
| **bsky-headline-editor** | Sets headlines, priorities, and story roles |

Agent configurations are in `.claude/agents/`.

## Output Formats

The `compile` command generates two output files in the workspace directory:

- **`digest.md`** - Markdown format with sections, headlines, and post links
- **`digest.html`** - HTML rendering with embedded media and external link cards

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

Runs 112 tests with Go's race detector enabled (`-race -count=3`).

### Clean

```bash
make clean
```

## Security

- Credentials are stored in macOS Keychain, not in code
- App passwords are scoped and revokable
- The agent never sees your main Bluesky password
