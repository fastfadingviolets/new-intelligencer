package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================
// extractRkeyFromURI Tests
// ============================================

func TestExtractRkeyFromURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{
			name:     "valid AT URI",
			uri:      "at://did:plc:xyz/app.bsky.feed.post/abc123",
			expected: "abc123",
		},
		{
			name:     "URI with trailing slash",
			uri:      "at://did:plc:xyz/app.bsky.feed.post/",
			expected: "",
		},
		{
			name:     "empty string",
			uri:      "",
			expected: "",
		},
		{
			name:     "no slashes returns input",
			uri:      "just-a-string",
			expected: "just-a-string",
		},
		{
			name:     "single slash",
			uri:      "foo/bar",
			expected: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRkeyFromURI(tt.uri)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================
// BuildThreadGraph Tests
// ============================================

func TestBuildThreadGraph_Empty(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{},
	}

	wd.BuildThreadGraph()

	assert.NotNil(t, wd.ThreadReplies)
	assert.Empty(t, wd.ThreadReplies)
}

func TestBuildThreadGraph_NoReplies(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "post1", ReplyTo: nil},
			{Rkey: "post2", ReplyTo: nil},
			{Rkey: "post3", ReplyTo: nil},
		},
	}

	wd.BuildThreadGraph()

	assert.NotNil(t, wd.ThreadReplies)
	assert.Empty(t, wd.ThreadReplies)
}

func TestBuildThreadGraph_SimpleReply(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "parent", ReplyTo: nil},
			{Rkey: "child", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/parent"}},
		},
	}

	wd.BuildThreadGraph()

	assert.Len(t, wd.ThreadReplies, 1)
	assert.Equal(t, []string{"child"}, wd.ThreadReplies["parent"])
}

func TestBuildThreadGraph_MultipleRepliesToSameParent(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "parent", ReplyTo: nil},
			{Rkey: "child1", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/parent"}},
			{Rkey: "child2", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/parent"}},
			{Rkey: "child3", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/parent"}},
		},
	}

	wd.BuildThreadGraph()

	assert.Len(t, wd.ThreadReplies, 1)
	assert.Len(t, wd.ThreadReplies["parent"], 3)
	assert.Contains(t, wd.ThreadReplies["parent"], "child1")
	assert.Contains(t, wd.ThreadReplies["parent"], "child2")
	assert.Contains(t, wd.ThreadReplies["parent"], "child3")
}

func TestBuildThreadGraph_ReplyChain(t *testing.T) {
	// A ← B ← C (C replies to B, B replies to A)
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "A", ReplyTo: nil},
			{Rkey: "B", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/A"}},
			{Rkey: "C", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/B"}},
		},
	}

	wd.BuildThreadGraph()

	assert.Len(t, wd.ThreadReplies, 2)
	assert.Equal(t, []string{"B"}, wd.ThreadReplies["A"])
	assert.Equal(t, []string{"C"}, wd.ThreadReplies["B"])
}

func TestBuildThreadGraph_OrphanedReply(t *testing.T) {
	// Reply to a post that's not in the dataset
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "orphan", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/external"}},
		},
	}

	wd.BuildThreadGraph()

	// Should still map the relationship even if parent isn't in Posts
	assert.Len(t, wd.ThreadReplies, 1)
	assert.Equal(t, []string{"orphan"}, wd.ThreadReplies["external"])
}

func TestBuildThreadGraph_EmptyURI(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "post", ReplyTo: &ReplyTo{URI: ""}},
		},
	}

	wd.BuildThreadGraph()

	// Empty URI should be skipped
	assert.Empty(t, wd.ThreadReplies)
}

// ============================================
// GetReplies Tests
// ============================================

func TestGetReplies_LazyInit(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "parent", ReplyTo: nil},
			{Rkey: "child", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/parent"}},
		},
		ThreadReplies: nil, // Not built yet
	}

	// GetReplies should build the graph on first call
	replies := wd.GetReplies("parent")

	assert.NotNil(t, wd.ThreadReplies)
	assert.Equal(t, []string{"child"}, replies)
}

func TestGetReplies_ReturnsReplies(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "parent", ReplyTo: nil},
			{Rkey: "child1", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/parent"}},
			{Rkey: "child2", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/parent"}},
		},
	}
	wd.BuildThreadGraph()

	replies := wd.GetReplies("parent")

	assert.Len(t, replies, 2)
}

func TestGetReplies_ReturnsNilForNoReplies(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "lonely", ReplyTo: nil},
		},
	}
	wd.BuildThreadGraph()

	replies := wd.GetReplies("lonely")

	assert.Nil(t, replies)
}

func TestGetReplies_ReturnsNilForUnknownParent(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{},
	}
	wd.BuildThreadGraph()

	replies := wd.GetReplies("nonexistent")

	assert.Nil(t, replies)
}

// ============================================
// IsReply Tests
// ============================================

func TestIsReply_True(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "reply", ReplyTo: &ReplyTo{URI: "at://did:plc:xyz/app.bsky.feed.post/parent"}},
		},
		Index: PostsIndex{"reply": 0},
	}

	assert.True(t, wd.IsReply("reply"))
}

func TestIsReply_False(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{
			{Rkey: "original", ReplyTo: nil},
		},
		Index: PostsIndex{"original": 0},
	}

	assert.False(t, wd.IsReply("original"))
}

func TestIsReply_UnknownRkey(t *testing.T) {
	wd := &WorkspaceData{
		Posts: []Post{},
		Index: PostsIndex{},
	}

	assert.False(t, wd.IsReply("nonexistent"))
}

// ============================================
// GenerateWorkspaceDir Tests
// ============================================

func TestGenerateWorkspaceDir(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected string
	}{
		{
			name:     "standard date",
			date:     time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC),
			expected: "digest-20-11-2025",
		},
		{
			name:     "single digit day and month",
			date:     time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			expected: "digest-05-01-2025",
		},
		{
			name:     "end of year",
			date:     time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			expected: "digest-31-12-2025",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateWorkspaceDir(tt.date)
			assert.Equal(t, tt.expected, result)
		})
	}
}
