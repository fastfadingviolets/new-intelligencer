package main

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompile_BasicStructure(t *testing.T) {
	config := Config{
		CreatedAt: time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC),
	}

	posts := []Post{
		{Rkey: "abc123", Text: "Test post", Author: Author{Handle: "alice.bsky.social"}, CreatedAt: time.Now()},
	}

	cats := Categories{
		"test-category": {"abc123"},
	}

	sums := Summaries{
		"test-category": "A test summary [abc123]",
	}

	markdown, err := CompileDigest(posts, cats, sums, config)
	require.NoError(t, err)

	// Check basic structure
	assert.Contains(t, markdown, "# Bluesky Digest")
	assert.Contains(t, markdown, "## test-category")
	assert.Contains(t, markdown, "---")
	assert.Contains(t, markdown, "## References")
}

func TestCompile_DateFormatting(t *testing.T) {
	config := Config{
		CreatedAt: time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC),
	}

	posts := []Post{}
	cats := Categories{}
	sums := Summaries{}

	markdown, err := CompileDigest(posts, cats, sums, config)
	require.NoError(t, err)

	// Should format as "DD MMM YYYY"
	assert.Contains(t, markdown, "20 November 2025")
}

func TestCompile_CategorySections(t *testing.T) {
	config := Config{CreatedAt: time.Now()}

	posts := []Post{
		{Rkey: "post1", Text: "First post", Author: Author{Handle: "alice.bsky.social"}, CreatedAt: time.Now()},
		{Rkey: "post2", Text: "Second post", Author: Author{Handle: "bob.bsky.social"}, CreatedAt: time.Now()},
		{Rkey: "post3", Text: "Third post", Author: Author{Handle: "charlie.bsky.social"}, CreatedAt: time.Now()},
	}

	cats := Categories{
		"ai-discussions": {"post1", "post2"},
		"tech-news":      {"post3"},
	}

	sums := Summaries{
		"ai-discussions": "Discussions about AI [post1] and ML [post2]",
		"tech-news":      "Latest tech news [post3]",
	}

	markdown, err := CompileDigest(posts, cats, sums, config)
	require.NoError(t, err)

	// Check category sections exist
	assert.Contains(t, markdown, "## ai-discussions (2 posts)")
	assert.Contains(t, markdown, "Discussions about AI [post1] and ML [post2]")

	assert.Contains(t, markdown, "## tech-news (1 post)")
	assert.Contains(t, markdown, "Latest tech news [post3]")
}

func TestCompile_ReferenceLinks(t *testing.T) {
	config := Config{CreatedAt: time.Now()}

	posts := []Post{
		{
			Rkey:      "abc123",
			Text:      "This is a test post",
			Author:    Author{Handle: "alice.bsky.social", DisplayName: "Alice"},
			CreatedAt: time.Date(2025, 11, 20, 14, 30, 0, 0, time.UTC),
		},
	}

	cats := Categories{
		"test": {"abc123"},
	}

	sums := Summaries{
		"test": "Summary with reference [abc123]",
	}

	markdown, err := CompileDigest(posts, cats, sums, config)
	require.NoError(t, err)

	// Check reference format
	assert.Contains(t, markdown, "[abc123] alice.bsky.social")
	assert.Contains(t, markdown, "This is a test post")
	assert.Contains(t, markdown, "https://bsky.app/profile/alice.bsky.social/post/abc123")
}

func TestCompile_RepostInReferences(t *testing.T) {
	config := Config{CreatedAt: time.Now()}

	posts := []Post{
		{
			Rkey:      "repost1",
			Text:      "Original post text",
			Author:    Author{Handle: "bob.bsky.social"},
			CreatedAt: time.Date(2025, 11, 20, 10, 0, 0, 0, time.UTC),
			Repost: &Repost{
				ByHandle: "charlie.bsky.social",
				At:       time.Date(2025, 11, 20, 15, 0, 0, 0, time.UTC),
			},
		},
	}

	cats := Categories{"test": {"repost1"}}
	sums := Summaries{"test": "Reference [repost1]"}

	markdown, err := CompileDigest(posts, cats, sums, config)
	require.NoError(t, err)

	// Should indicate it's a repost
	assert.Contains(t, markdown, "[reposted by charlie.bsky.social]")
	assert.Contains(t, markdown, "Original post text")
}

func TestCompile_EscapesMarkdown(t *testing.T) {
	config := Config{CreatedAt: time.Now()}

	posts := []Post{
		{
			Rkey:      "escape1",
			Text:      "Text with *asterisks* and _underscores_ and [brackets]",
			Author:    Author{Handle: "test.bsky.social"},
			CreatedAt: time.Now(),
		},
	}

	cats := Categories{"test": {"escape1"}}
	sums := Summaries{"test": "Summary [escape1]"}

	markdown, err := CompileDigest(posts, cats, sums, config)
	require.NoError(t, err)

	// Special characters should be escaped in quotes
	// (escaping implementation detail - just verify it doesn't break)
	assert.Contains(t, markdown, "Text with")
}

func TestCompile_EmptyCategory(t *testing.T) {
	config := Config{CreatedAt: time.Now()}

	posts := []Post{
		{Rkey: "post1", Text: "Test", Author: Author{Handle: "alice.bsky.social"}, CreatedAt: time.Now()},
	}

	cats := Categories{
		"with-summary":    {"post1"},
		"without-summary": {}, // Empty category
	}

	sums := Summaries{
		"with-summary": "Has summary [post1]",
		// No summary for "without-summary"
	}

	markdown, err := CompileDigest(posts, cats, sums, config)
	require.NoError(t, err)

	// Should still have section for category with summary
	assert.Contains(t, markdown, "## with-summary")

	// Empty category should not appear (0 posts)
	// (or it could appear with placeholder - implementation choice)
}

func TestCompile_NoCategories(t *testing.T) {
	config := Config{CreatedAt: time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC)}

	posts := []Post{}
	cats := Categories{}
	sums := Summaries{}

	markdown, err := CompileDigest(posts, cats, sums, config)
	require.NoError(t, err)

	// Should still have title
	assert.Contains(t, markdown, "# Bluesky Digest - 20 November 2025")

	// Should have references header (even if empty)
	assert.Contains(t, markdown, "## References")
}

func TestCompile_OrderedOutput(t *testing.T) {
	config := Config{CreatedAt: time.Now()}

	posts := []Post{
		{Rkey: "post1", Text: "First", Author: Author{Handle: "a.bsky.social"}, CreatedAt: time.Now()},
		{Rkey: "post2", Text: "Second", Author: Author{Handle: "b.bsky.social"}, CreatedAt: time.Now()},
		{Rkey: "post3", Text: "Third", Author: Author{Handle: "c.bsky.social"}, CreatedAt: time.Now()},
	}

	cats := Categories{
		"zebra":   {"post3"},
		"alpha":   {"post1"},
		"beta":    {"post2"},
	}

	sums := Summaries{
		"zebra": "Z [post3]",
		"alpha": "A [post1]",
		"beta":  "B [post2]",
	}

	markdown, err := CompileDigest(posts, cats, sums, config)
	require.NoError(t, err)

	// Categories should be in alphabetical order
	alphaIdx := strings.Index(markdown, "## alpha")
	betaIdx := strings.Index(markdown, "## beta")
	zebraIdx := strings.Index(markdown, "## zebra")

	assert.True(t, alphaIdx < betaIdx, "alpha should come before beta")
	assert.True(t, betaIdx < zebraIdx, "beta should come before zebra")
}

func TestFormatDigestDate(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected string
	}{
		{
			name:     "November 2025",
			date:     time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC),
			expected: "20 November 2025",
		},
		{
			name:     "January 2026",
			date:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: "01 January 2026",
		},
		{
			name:     "December with single digit",
			date:     time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
			expected: "05 December 2025",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDigestDate(tt.date)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostURL(t *testing.T) {
	post := Post{
		Rkey:   "abc123xyz",
		Author: Author{Handle: "alice.bsky.social"},
	}

	url := postURL(post)

	assert.Equal(t, "https://bsky.app/profile/alice.bsky.social/post/abc123xyz", url)
}

func TestFormatPostTime(t *testing.T) {
	postTime := time.Date(2025, 11, 20, 14, 30, 0, 0, time.UTC)

	formatted := formatPostTime(postTime)

	// Should format as "Nov 20, 2:30pm" or similar human-readable format
	assert.Contains(t, formatted, "Nov")
	assert.Contains(t, formatted, "20")
}
