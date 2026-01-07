First, initialize the digest workspace and fetch posts:

```bash
./bin/digest init
./bin/digest fetch
```

Then invoke subagents in sequence.

**IMPORTANT:** All subagents run from the **PROJECT ROOT** directory (where `bin/`, `newspaper.json`, and `digest-*/` are located). They should NEVER `cd` into any directory.

## Step 1: Parallel Batch Categorization

1. Run `./bin/digest status` to get the total post count
2. Calculate batches of ~100 posts each
3. Spawn one `bsky-section-categorizer` agent per batch IN PARALLEL, passing the offset and limit:
   - Agent 1: `--offset 0 --limit 100`
   - Agent 2: `--offset 100 --limit 100`
   - Agent 3: `--offset 200 --limit 100`
   - etc.

Each agent categorizes its batch into all sections (except front-page). Wait for all to complete.

## Step 2: Story Consolidation

After all section categorizers complete, consolidate related posts into stories:

1. Run `./bin/digest list-categories --with-counts` to see which sections have posts
2. For each news section with posts, spawn a `bsky-consolidator` agent IN PARALLEL:
   - Each agent processes one section
   - Groups related posts into story groups
   - Sets draft headlines (optional)

Wait for all consolidators to complete.

## Step 3: Front Page Selection

After consolidation is complete:

```
bsky-front-page-selector - pick 4-6 top STORIES (not individual posts) for front page
```

This agent moves entire story groups to the front-page section.

## Step 4: Headlines & Priorities (Parallel)

After front page is selected:

1. Run `./bin/digest auto-group-remaining` to wrap ungrouped posts into single-post stories
2. Run `./bin/digest list-categories --with-counts` to see which sections have stories
3. For each section with stories, spawn a `bsky-headline-editor` agent IN PARALLEL:
   - Each agent processes ONE section
   - Sets headline and priority for EVERY story in that section
   - Verifies completion with `./bin/digest show-unprocessed <section-id>`

Wait for all headline editors to complete.

## Step 5: Compile

After all headlines and priorities are set:

```bash
./bin/digest compile
```

**Compile will FAIL if any stories are missing headlines or priorities.** If it fails, check `./bin/digest show-unprocessed` and re-run headline editors for incomplete sections.

The digest workspace will be in `digest-DD-MM-YYYY/` directory with all state files.
