First, initialize the digest workspace and fetch posts:

```bash
./bin/digest init
./bin/digest fetch --limit 1000
```

Then invoke subagents in sequence:

1. **bsky-categorizer** - categorize all posts into meaningful categories
2. **bsky-category-hider** - hide entire categories that are clearly not digest-worthy
3. **bsky-post-hider** - hide low-value posts within categories while preserving signal
4. **bsky-section-mapper** - map categories to newspaper sections, create story groups
5. **bsky-summarizer** - write summaries and compile the final digest

The digest workspace will be in `digest-DD-MM-YYYY/` directory with all state files.
