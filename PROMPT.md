First, initialize the digest workspace and fetch posts:

```bash
./bin/digest init
./bin/digest fetch --limit 1000
```

Then invoke subagents in sequence:

## Step 1: Parallel Section Categorization

Read `newspaper.json` to get the list of sections. For each section (except `front-page`), spawn a `bsky-section-categorizer` agent IN PARALLEL with the section ID.

Wait for all to complete. File locking ensures no data races.

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
