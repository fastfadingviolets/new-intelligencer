First, initialize the digest workspace and fetch posts:

```bash
./bin/digest init
./bin/digest fetch
```

Then invoke subagents in sequence.

**IMPORTANT:** All subagents run from the **PROJECT ROOT** directory (where `bin/`, `newspaper.json`, and `digest-*/` are located). They should NEVER `cd` into any directory.

**IMPORTANT:** Always spawn agents with `run_in_background: true`. This keeps agent outputs out of your context.

## Waiting for Stage Completion

Each agent marks itself done when finished. Use `--wait-for` to block until a stage completes.

**CRITICAL:** Run `--wait-for` commands in the **FOREGROUND** with a **10-minute timeout** (600000ms). Do NOT background these commands. Do NOT poll or check status manually. Just wait for the command to return:

```bash
./bin/digest status --wait-for categorization   # blocks until all batches done
./bin/digest status --wait-for consolidation    # blocks until all sections consolidated
./bin/digest status --wait-for front-page       # blocks until front page selected
./bin/digest status --wait-for headlines        # blocks until all headlines set
```

The command prints progress and exits when the stage completes. If the Bash tool times out after 10 minutes, simply **re-run the same `--wait-for` command**. It will resume waiting. Keep re-running until the stage completes. Do NOT try alternative polling approaches.

## Step 1: Parallel Batch Categorization

1. Run `./bin/digest status` to get the total post count
2. Calculate batches of ~100 posts each
3. Spawn one `bsky-section-categorizer` agent per batch IN PARALLEL, passing the offset and limit:
   - Agent 1: `--offset 0 --limit 100`
   - Agent 2: `--offset 100 --limit 100`
   - Agent 3: `--offset 200 --limit 100`
   - etc.

Each agent categorizes its batch into all sections (except front-page). After spawning all agents, run:

```bash
./bin/digest status --wait-for categorization
```

## Step 2: Story Consolidation

After all section categorizers complete, consolidate related posts into stories:

1. Run `./bin/digest list-categories --with-counts` to see which sections have posts
2. For each news section with posts, spawn a `bsky-consolidator` agent IN PARALLEL:
   - Each agent processes one section
   - Groups related posts into story groups
   - Sets draft headlines (optional)

After spawning all agents, run:

```bash
./bin/digest status --wait-for consolidation
```

## Step 3: Front Page Selection

After consolidation is complete:

```
bsky-front-page-selector - pick 4-6 top STORIES (not individual posts) for front page
```

This agent moves entire story groups to the front-page section. After spawning the agent, run:

```bash
./bin/digest status --wait-for front-page
```

## Step 4: Headlines & Priorities (Parallel)

After front page is selected:

1. Run `./bin/digest auto-group-remaining` to wrap ungrouped posts into single-post stories
2. Run `./bin/digest list-categories --with-counts` to see which sections have stories
3. For each section with stories, spawn a `bsky-headline-editor` agent IN PARALLEL:
   - Each agent processes ONE section
   - Sets headline and priority for EVERY story in that section
   - Verifies completion with `./bin/digest show-unprocessed <section-id>`

After spawning all agents, run:

```bash
./bin/digest status --wait-for headlines
```

## Step 5: Compile

After all headlines and priorities are set:

```bash
./bin/digest compile
```

**Compile will FAIL if any stories are missing headlines or priorities.** If it fails, check `./bin/digest show-unprocessed` and re-run headline editors for incomplete sections.

The digest workspace will be in `digest-DD-MM-YYYY/` directory with all state files.
