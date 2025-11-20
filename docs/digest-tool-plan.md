# Digest Tool Design & Implementation Plan

## Overview

The `digest` CLI tool provides a programmatic interface for fetching, categorizing, and summarizing Bluesky posts. It replaces the previous jq-based approach with simple, idempotent commands that agents can use reliably.

## Architecture Principles

1. **Dual Format Approach**: Posts are stored in full detail (storage format) but displayed to agents in a minimal format
2. **Idempotent Operations**: All commands can be run multiple times safely
3. **File-Based State**: All data stored in inspectable JSON files
4. **Date-Named Workspaces**: Each digest lives in `digest-DD-MM-YYYY/`
5. **Fast Lookups**: Posts indexed by rkey for O(1) access
6. **Agent-Friendly**: Minimal JSON output reduces token usage for Haiku

---

## Directory Structure

```
digest-20-11-2025/
├── config.json        # Workspace configuration
├── posts.json         # All posts (storage format - complete data)
├── posts-index.json   # Fast lookup: rkey → array index
├── categories.json    # Category → [rkeys] mapping
└── summaries.json     # Category → summary text mapping
```

---

## Data Formats

### Storage Format (posts.json)

**Purpose**: Store complete post data as fetched from API

**Structure**: Array of posts (chronological order)

**Fields**: All available data from Bluesky API

```json
[
  {
    "rkey": "3lbkj2x3abcd",
    "uri": "at://did:plc:xyz/app.bsky.feed.post/3lbkj2x3abcd",
    "cid": "bafyreiabc123...",
    "text": "This is the full post text about AI safety...",
    "author": {
      "did": "did:plc:xyz123",
      "handle": "alice.bsky.social",
      "display_name": "Alice"
    },
    "created_at": "2025-11-20T08:15:00Z",
    "indexed_at": "2025-11-20T08:15:30Z"
  },
  {
    "rkey": "4mcxk3y4defg",
    "uri": "at://did:plc:abc/app.bsky.feed.post/4mcxk3y4defg",
    "cid": "bafyreidef456...",
    "text": "Apple announced new privacy features",
    "author": {
      "did": "did:plc:abc456",
      "handle": "bob.bsky.social",
      "display_name": "Bob"
    },
    "created_at": "2025-11-20T09:22:00Z",
    "indexed_at": "2025-11-20T09:22:15Z",
    "repost": {
      "by_did": "did:plc:def789",
      "by_handle": "charlie.bsky.social",
      "at": "2025-11-20T09:45:00Z"
    }
  },
  {
    "rkey": "5ndyl4z5ghij",
    "uri": "at://did:plc:def/app.bsky.feed.post/5ndyl4z5ghij",
    "cid": "bafyreighi789...",
    "text": "@someone I totally agree with this take",
    "author": {
      "did": "did:plc:def789",
      "handle": "dave.bsky.social",
      "display_name": "Dave"
    },
    "created_at": "2025-11-20T09:30:00Z",
    "indexed_at": "2025-11-20T09:30:45Z",
    "reply_to": {
      "uri": "at://did:plc:xyz/app.bsky.feed.post/2kabc123",
      "author_handle": "someone.bsky.social"
    },
    "images": [
      {
        "url": "https://cdn.bsky.app/img/feed_thumbnail/...",
        "alt": "Screenshot of the discussion"
      }
    ]
  }
]
```

**Field Rules**:
- **Always present**: rkey, uri, cid, text, author, created_at, indexed_at
- **Optional** (absent if not applicable): repost, reply_to, images
- **rkey**: Extracted from URI (last path segment after `/post/`)

### Display Format (for agents)

**Purpose**: Minimal JSON for agent consumption (reduces tokens)

**Structure**: Array of posts with only essential fields

**Fields**: Excludes uri, cid, author.did

```json
[
  {
    "rkey": "3lbkj2x3abcd",
    "text": "This is the full post text about AI safety...",
    "author": {
      "handle": "alice.bsky.social",
      "display_name": "Alice"
    },
    "created_at": "2025-11-20T08:15:00Z"
  },
  {
    "rkey": "4mcxk3y4defg",
    "text": "Apple announced new privacy features",
    "author": {
      "handle": "bob.bsky.social",
      "display_name": "Bob"
    },
    "created_at": "2025-11-20T09:22:00Z",
    "repost": {
      "by_handle": "charlie.bsky.social",
      "at": "2025-11-20T09:45:00Z"
    }
  },
  {
    "rkey": "5ndyl4z5ghij",
    "text": "@someone I totally agree with this take",
    "author": {
      "handle": "dave.bsky.social",
      "display_name": "Dave"
    },
    "created_at": "2025-11-20T09:30:00Z",
    "reply_to": {
      "author_handle": "someone.bsky.social"
    },
    "images": [
      {
        "url": "https://cdn.bsky.app/img/feed_thumbnail/...",
        "alt": "Screenshot of the discussion"
      }
    ]
  }
]
```

**Conversion Rules**:
- Remove: `uri`, `cid`, `indexed_at`, `author.did`, `repost.by_did`, `reply_to.uri`
- Keep: `rkey`, `text`, `author.handle`, `author.display_name`, `created_at`, `repost`, `reply_to`, `images`
- Omit optional fields when absent (no empty objects/arrays)

### Index Format (posts-index.json)

**Purpose**: Fast O(1) lookup from rkey to array index

**Structure**: Object mapping rkey → index

```json
{
  "3lbkj2x3abcd": 0,
  "4mcxk3y4defg": 1,
  "5ndyl4z5ghij": 2
}
```

**Maintenance**: Built during fetch, kept in sync with posts.json

### Categories Format (categories.json)

**Purpose**: Track which posts belong to which categories

**Structure**: Object mapping category name → array of rkeys

```json
{
  "ai-discussions": ["3lbkj2x3abcd", "6oezm5a6klmn", "9rgcp8d9uvwx"],
  "tech-news": ["4mcxk3y4defg", "7pfan6b7opqr"],
  "meta-bsky": ["5ndyl4z5ghij", "8qgbo7c8rstu"],
  "uncategorized": ["adummy9z0yzab"]
}
```

**Rules**:
- Each post belongs to exactly one category
- Category names are kebab-case
- Empty categories are removed during operations

### Summaries Format (summaries.json)

**Purpose**: Store narrative summaries for each category

**Structure**: Object mapping category name → summary text

```json
{
  "ai-discussions": "Several conversations emerged about LLMs and safety alignment, with alice discussing constitutional AI [3lbkj2x3abcd] and bob sharing research on RLHF [6oezm5a6klmn]. The thread continued with technical details [9rgcp8d9uvwx].",
  "tech-news": "Apple announced new privacy features [4mcxk3y4defg] which sparked discussion about user control and data encryption standards [7pfan6b7opqr].",
  "meta-bsky": "Community members discussed new Bluesky features [5ndyl4z5ghij] and protocol improvements [8qgbo7c8rstu]."
}
```

### Config Format (config.json)

**Purpose**: Store workspace metadata

```json
{
  "version": "1",
  "created_at": "2025-11-20T00:00:00Z",
  "time_range": {
    "since": "2025-11-19T00:00:00Z",
    "until": null
  }
}
```

---

## CLI Commands

### `digest init [--since TIMESTAMP] [--dir DIR]`

**Purpose**: Create new digest workspace

**Options**:
- `--since`: Start time for fetching (default: 24h ago)
- `--dir`: Custom directory name (default: auto-generated `digest-DD-MM-YYYY`)

**Behavior**:
1. Generate directory name from current date (DD-MM-YYYY format)
2. Create directory structure
3. Initialize config.json with time range
4. Create empty posts.json as `[]`
5. Create empty posts-index.json as `{}`
6. Create empty categories.json as `{}`
7. Create empty summaries.json as `{}`

**Idempotency**: Will not overwrite existing workspace (error if exists)

**Example**:
```bash
digest init
# Creates: digest-20-11-2025/

digest init --since 2025-11-15T00:00:00Z
# Creates workspace for posts since Nov 15
```

### `digest fetch [--limit N]`

**Purpose**: Fetch posts from Bluesky timeline in one operation

**Options**:
- `--limit`: Max total posts to fetch (default: unlimited)

**Behavior**:
1. Authenticate with Bluesky using env vars (BSKY_HANDLE, BSKY_PASSWORD)
2. Fetch posts in batches (cursor pagination)
3. Filter by time range from config.json
4. Extract rkey from each post URI
5. Add to posts.json (check for duplicates by URI)
6. Build posts-index.json concurrently
7. Show progress bar: "Fetching... 142/??? posts"
8. Continue until: time boundary reached OR no more posts OR limit reached

**Idempotency**: Checks URI before adding (no duplicates)

**Example**:
```bash
digest fetch
# Fetches all posts in time range

digest fetch --limit 50
# Fetches max 50 posts (useful for testing)
```

**Progress Output**:
```
Authenticating with bsky.social...
Fetching posts... ████████████████████ 142 posts
Building index...
Done! Fetched 142 posts to digest-20-11-2025/
```

### `digest read-posts [--offset N] [--limit M] [--format json|compact]`

**Purpose**: Display posts in minimal format for agents

**Options**:
- `--offset`: Skip first N posts (default: 0)
- `--limit`: Show M posts (default: 20)
- `--format`: Output format (default: json)

**Behavior**:
1. Load posts.json
2. Apply offset and limit (array slicing)
3. Convert to display format
4. Output as JSON array or compact text

**Format Examples**:

**JSON** (default):
```json
[
  {
    "rkey": "3lbkj2x3abcd",
    "text": "Post text here...",
    "author": {"handle": "alice.bsky.social", "display_name": "Alice"},
    "created_at": "2025-11-20T08:15:00Z"
  }
]
```

**Compact**:
```
[3lbkj2x3abcd] alice.bsky.social (Nov 20, 8:15am)
Post text here...

[4mcxk3y4defg] bob.bsky.social (Nov 20, 9:22am) [reposted by charlie]
Apple announced new privacy features
```

**Example Usage**:
```bash
# View first 10 posts
digest read-posts --limit 10

# View next 10 posts
digest read-posts --offset 10 --limit 10

# Compact format for human reading
digest read-posts --limit 5 --format compact
```

### `digest show-category <category> [--format json|compact]`

**Purpose**: Display all posts in a specific category

**Behavior**:
1. Load categories.json
2. Get rkeys for category
3. Use posts-index.json to lookup posts
4. Convert to display format
5. Output (same format options as read-posts)

**Example**:
```bash
digest show-category ai-discussions
digest show-category tech-news --format compact
```

### `digest categorize <category> <rkey> [<rkey>...]`

**Purpose**: Assign posts to a category (idempotent)

**Behavior**:
1. Validate all rkeys exist in posts-index.json (fail if any invalid)
2. Load categories.json
3. For each rkey:
   - Remove from any existing category
   - Add to target category
4. Create target category if doesn't exist
5. Save categories.json

**Idempotency**: Running same command twice produces same result

**Example**:
```bash
# Categorize multiple posts at once
digest categorize ai-discussions 3lbkj2x3abcd 6oezm5a6klmn 9rgcp8d9uvwx

# Move post to different category
digest categorize tech-news 3lbkj2x3abcd  # Moves from ai-discussions
```

**Output**:
```
Categorized 3 posts into 'ai-discussions'
  3lbkj2x3abcd, 6oezm5a6klmn, 9rgcp8d9uvwx
```

### `digest list-categories [--with-counts]`

**Purpose**: Show all categories

**Output**:
```
ai-discussions (12 posts)
tech-news (8 posts)
meta-bsky (5 posts)
```

### `digest merge-categories <from> <to>`

**Purpose**: Consolidate two categories

**Behavior**:
1. Load categories.json and summaries.json
2. Move all rkeys from `<from>` to `<to>`
3. Delete `<from>` entry from categories.json
4. Delete `<from>` entry from summaries.json (if exists)
5. Save both files

**Idempotency**: Safe to run if source already merged (no-op)

**Example**:
```bash
digest merge-categories llm-discussion ai-discussions
```

**Output**:
```
Merged 'llm-discussion' (3 posts) into 'ai-discussions'
ai-discussions now has 15 posts
```

### `digest write-summary <category> <text>`

**Purpose**: Write narrative summary for a category

**Behavior**:
1. Validate category exists
2. Update summaries.json[category] = text
3. Save summaries.json

**Idempotency**: Overwrites existing summary

**Example**:
```bash
digest write-summary ai-discussions "Several conversations emerged about LLMs..."
```

### `digest compile [--output FILE]`

**Purpose**: Generate final markdown digest

**Options**:
- `--output`: Output file path (default: `digest.md` in workspace)

**Behavior**:
1. Load all data files
2. Generate markdown structure:
   - Title: "# Bluesky Digest - DD MMM YYYY"
   - For each category (sorted alphabetically):
     - Section header: "## Category Name (N posts)"
     - Summary text (from summaries.json)
   - References section:
     - "[rkey] handle (date time): quote"
     - Link to bsky.app post
3. Write to output file

**Markdown Structure**:
```markdown
# Bluesky Digest - 20 November 2025

## AI Discussions (12 posts)

Several conversations emerged about LLMs and safety alignment, with alice
discussing constitutional AI [3lbkj2x3abcd] and bob sharing research on RLHF
[6oezm5a6klmn]. The thread continued with technical details about reward
modeling [9rgcp8d9uvwx].

## Tech News (8 posts)

Apple announced new privacy features [4mcxk3y4defg] which sparked discussion
about user control. Several people shared articles about browser security
[7pfan6b7opqr].

---

## References

[3lbkj2x3abcd] alice.bsky.social (Nov 20, 8:15am)
"Constitutional AI is an interesting approach to AI safety..."
https://bsky.app/profile/alice.bsky.social/post/3lbkj2x3abcd

[4mcxk3y4defg] bob.bsky.social (Nov 20, 9:22am) [reposted by charlie]
"Apple's new privacy dashboard is impressive"
https://bsky.app/profile/bob.bsky.social/post/4mcxk3y4defg
```

**Example**:
```bash
digest compile
# Writes to digest-20-11-2025/digest.md

digest compile --output final-digest.md
# Writes to custom location
```

### `digest status`

**Purpose**: Show workspace status overview

**Output**:
```
Digest: digest-20-11-2025/
Time Range: Nov 19-20, 2025

Posts: 142 total
  Categorized: 140 (98.6%)
  Uncategorized: 2 (1.4%)

Categories: 4
  ai-discussions: 12 posts
  tech-news: 8 posts
  meta-bsky: 5 posts
  personal: 115 posts

Summaries: 3/4 complete (75%)
  Missing: personal

Compiled: digest.md (exists)
```

### `digest uncategorized [--format json|compact]`

**Purpose**: Show posts not in any category

**Behavior**:
1. Load posts.json and categories.json
2. Find rkeys not in any category
3. Lookup posts and display

**Example**:
```bash
digest uncategorized
digest uncategorized --format compact
```

---

## Agent Workflows

### Categorizer Agent

**Old workflow** (jq-based):
```bash
# Complex jq commands to manipulate JSON
jq '.categories["ai-discussions"] += [...]' digest-work.json
```

**New workflow** (digest commands):
```bash
# Read posts in batches
digest read-posts --offset 0 --limit 50

# Decide categories and assign
digest categorize ai-discussions 3lbkj2x3abcd 6oezm5a6klmn
digest categorize tech-news 4mcxk3y4defg 7pfan6b7opqr

# Check progress
digest status

# Continue with next batch
digest read-posts --offset 50 --limit 50
```

### Consolidator Agent

**Old workflow**:
```bash
# Complex jq merges
jq '.categories["primary"] += .categories["duplicate"] | del(.categories["duplicate"])' ...
```

**New workflow**:
```bash
# Review categories
digest list-categories --with-counts

# Merge similar ones
digest merge-categories llm-talk ai-discussions
digest merge-categories bsky-meta meta-bsky
```

### Summarizer Agent

**Old workflow**:
```bash
# Read JSON, manually construct markdown
cat digest-work.json | jq ... > digest.md
```

**New workflow**:
```bash
# Review each category
digest show-category ai-discussions
# Write summary
digest write-summary ai-discussions "Several conversations emerged..."

# Repeat for all categories
digest show-category tech-news
digest write-summary tech-news "Apple announced..."

# Generate final digest
digest compile
```

---

## Implementation Structure

### Go Project Layout

```
plumbing/digest/
├── main.go              # CLI entry point, cobra setup
├── types.go             # Data structures (Post, Config, etc.)
├── storage.go           # JSON file I/O (Load/Save)
├── storage_test.go
├── display.go           # Format conversion (storage → display)
├── display_test.go
├── categories.go        # Category operations
├── categories_test.go
├── fetch.go             # Bluesky API client
├── fetch_test.go
├── compile.go           # Markdown generation
├── compile_test.go
├── cmd_init.go          # init command
├── cmd_init_test.go
├── cmd_fetch.go         # fetch command
├── cmd_fetch_test.go
├── cmd_read.go          # read-posts command
├── cmd_categorize.go    # categorize command
├── cmd_compile.go       # compile command
├── cmd_status.go        # status command
├── go.mod
├── go.sum
└── testdata/
    ├── api_responses.json    # Mock API responses
    ├── sample_posts.json     # Test fixtures
    └── expected_output.md    # Expected markdown
```

### Key Dependencies

```go
// go.mod
module github.com/v/bsky-digest-agent/plumbing/digest

go 1.21

require (
    github.com/bluesky-social/indigo v0.x.x  // Bluesky API
    github.com/spf13/cobra v1.8.0           // CLI framework
    github.com/stretchr/testify v1.8.4      // Testing
    github.com/schollz/progressbar/v3 v3.x  // Progress bars
)
```

---

## Testing Strategy

### Unit Testing Approach

**Philosophy**: Test-Driven Development (TDD)
- Write tests first
- Implement to make tests pass
- Refactor with confidence

**Coverage Goals**:
- Storage: 100% (pure I/O)
- Display: 100% (pure functions)
- Categories: 100% (pure logic)
- Fetch: 90%+ (mocked API)
- Compile: 100% (string generation)
- CLI: 80%+ (integration-style)

### Test Fixtures

**Mock API Responses** (`testdata/api_responses.json`):
```json
{
  "timeline_page1": {
    "feed": [
      {
        "post": {
          "uri": "at://did:plc:test1/app.bsky.feed.post/3lbkj2x3abcd",
          "cid": "bafyrei...",
          "author": {
            "did": "did:plc:test1",
            "handle": "alice.bsky.social",
            "displayName": "Alice"
          },
          "record": {
            "text": "Test post about AI",
            "createdAt": "2025-11-20T08:15:00Z"
          }
        }
      }
    ],
    "cursor": "next123"
  },
  "timeline_page2": {
    "feed": [...],
    "cursor": null
  }
}
```

### Test Organization

#### Storage Tests (`storage_test.go`)
```go
func TestLoadPosts_EmptyFile(t *testing.T)
func TestLoadPosts_ValidData(t *testing.T)
func TestLoadPosts_InvalidJSON(t *testing.T)
func TestSavePosts_AtomicWrite(t *testing.T)
func TestLoadCategories_EmptyVsAbsent(t *testing.T)
func TestExtractRkey_FromURI(t *testing.T)
```

#### Display Tests (`display_test.go`)
```go
func TestFormatForDisplay_RemovesURICID(t *testing.T)
func TestFormatForDisplay_KeepsEssentials(t *testing.T)
func TestFormatForDisplay_PreservesRepostMetadata(t *testing.T)
func TestFormatForDisplay_OmitsAbsentFields(t *testing.T)
```

#### Category Tests (`categories_test.go`)
```go
func TestCategorize_NewCategory(t *testing.T)
func TestCategorize_MovesFromOldCategory(t *testing.T)
func TestCategorize_Idempotent(t *testing.T)
func TestMergeCategories_Basic(t *testing.T)
func TestMergeCategories_Idempotent(t *testing.T)
func TestShowCategory_LookupPosts(t *testing.T)
func TestUncategorized_FindsOrphans(t *testing.T)
```

#### Fetch Tests (`fetch_test.go`)
```go
// Uses mocked API client
func TestFetch_MockedAPI_SinglePage(t *testing.T)
func TestFetch_MockedAPI_Pagination(t *testing.T)
func TestFetch_MockedAPI_TimeFiltering(t *testing.T)
func TestConvertPost_FromAPI(t *testing.T)
func TestConvertPost_Repost(t *testing.T)
```

#### Compile Tests (`compile_test.go`)
```go
func TestCompile_BasicStructure(t *testing.T)
func TestCompile_DateFormatting(t *testing.T)
func TestCompile_ReferenceLinks(t *testing.T)
func TestCompile_EscapesMarkdown(t *testing.T)
```

### Running Tests

```bash
# All tests
cd plumbing/digest && go test -v ./...

# With coverage
go test -v -cover ./...

# Specific test
go test -v -run TestFormatForDisplay

# Watch mode (requires entr)
ls *.go | entr -c go test -v ./...
```

---

## Implementation Phases

### Phase 1: Setup ✅ COMPLETE
- [x] Create docs/digest-tool-plan.md
- [x] Remove .claude/agents/bsky-digest.md
- [x] Set up plumbing/digest/ directory
- [x] Initialize go.mod with dependencies (testify, cobra, indigo, progressbar)
- [x] Create types.go with all data structures

### Phase 2: Core Data (TDD) ✅ COMPLETE
- [x] Write storage tests → implement storage.go
- [x] Write display tests → implement display.go
- [x] Verify: `go test ./...` passes (22 tests, 100% pass rate)

### Phase 3: Categories (TDD) ✅ COMPLETE
- [x] Write category tests → implement categories.go
- [x] Verify: All category operations work correctly (41 tests, 100% pass rate)

### Phase 4: Fetch (Mocked) ✅ COMPLETE
- [x] Refactor existing Bluesky code into fetch.go
- [x] Write fetch tests (logic tests, integration tests skipped)
- [x] Verify: Fetch logic works (49 tests, 47 passed, 2 skipped for integration)
- [x] Note: Image extraction TODO - awaiting field name verification

### Phase 5: Compilation ✅ COMPLETE
- [x] Write compile tests with expected output
- [x] Implement markdown generation
- [x] Verify: Generated markdown matches spec (62 tests, 60 passed, 2 skipped)

### Phase 6: CLI Wiring ✅ COMPLETE
- [x] Implement cobra commands (all 11 commands)
- [x] Create workspace management helpers
- [x] Update Makefile to build ./bin/digest
- [x] Verify: All tests pass (62 tests, 60 passed, 2 skipped)
- [x] Verify: CLI help works and all commands available

### Phase 7: Manual Testing
- [ ] Run digest init
- [ ] Run digest fetch with real API
- [ ] Manually categorize posts
- [ ] Verify digest compile output

### Phase 8: Agent Integration
- [x] Update agent instruction files (bsky-categorizer.md, bsky-consolidator.md, bsky-summarizer.md)
- [x] Update PROMPT.md with digest CLI workflow
- [ ] Test full workflow with agents
- [ ] Verify final digest.md quality

---

## Success Criteria

### Functional Requirements
✓ All commands work as specified
✓ Idempotency verified for all operations
✓ Display format reduces token usage vs storage format
✓ Fast lookups via posts-index.json
✓ Generated markdown matches spec

### Quality Requirements
✓ 90%+ test coverage
✓ All tests pass
✓ No data loss (atomic writes)
✓ Clear error messages
✓ Performance: Fetch 1000 posts in <30s

### Integration Requirements
✓ Agents can categorize posts successfully
✓ Agents can consolidate categories
✓ Agents can write summaries
✓ Final digest.md is coherent and useful

---

## Future Enhancements

- [ ] `digest search <query>` - Full-text search in posts
- [ ] `digest stats` - Analytics (top authors, time distribution)
- [ ] `digest export` - Export to CSV/JSON for analysis
- [ ] `digest rename-category` - Rename without re-categorizing
- [ ] `digest validate` - Check workspace integrity
- [ ] `digest sync` - Fetch new posts into existing digest
- [ ] Progressive fetch with resume (if interrupted)
- [ ] Embedded thumbnails in markdown
- [ ] Thread reconstruction for replies

---

## Notes

- **No state.json**: Cursor is ephemeral during fetch (one big gulp)
- **rkey as ID**: Use full rkey from URI, not shortened version
- **Absent fields**: Optional fields omitted when not present (cleaner JSON)
- **One category per post**: Enforced by categorize command
- **Atomic writes**: All file operations use temp file + rename pattern
- **Date format**: DD-MM-YYYY for directory names (e.g., 20-11-2025)
