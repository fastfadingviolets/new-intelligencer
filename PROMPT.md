First, initialize the digest workspace and fetch posts:

```bash
./bin/digest init
./bin/digest fetch --limit 1000
```

Then invoke three subagents in sequence:

1. Use bsky-categorizer subagent to categorize all posts into meaningful categories
2. Use bsky-consolidator subagent to merge similar categories and delete irrelevant ones
3. Use bsky-summarizer subagent to write summaries and compile the final digest.md

The digest workspace will be in digest-DD-MM-YYYY/ directory with all state files.
