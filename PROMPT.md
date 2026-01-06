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

## Step 2: Front Page Selection

After all section categorizers complete:

```
bsky-front-page-selector - pick 4-6 top stories for front page
```

## Step 3: Story Editing

After front page is selected:

```
bsky-story-editor - create story groups, pick headlines, check primary links, compile
```

The digest workspace will be in `digest-DD-MM-YYYY/` directory with all state files.
