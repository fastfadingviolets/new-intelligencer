package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================
// Sorting Tests
// ============================================

func TestSortByLikes(t *testing.T) {
	posts := []Post{
		{Rkey: "low", LikeCount: 5},
		{Rkey: "high", LikeCount: 100},
		{Rkey: "medium", LikeCount: 50},
	}

	sorted := sortByLikes(posts)

	// Should be descending by likes
	assert.Equal(t, "high", sorted[0].Rkey)
	assert.Equal(t, "medium", sorted[1].Rkey)
	assert.Equal(t, "low", sorted[2].Rkey)

	// Original should be unchanged (immutable)
	assert.Equal(t, "low", posts[0].Rkey)
}

func TestSortByEngagement(t *testing.T) {
	posts := []Post{
		{Rkey: "low", ReplyCount: 1, RepostCount: 1},       // engagement = 2
		{Rkey: "high", ReplyCount: 50, RepostCount: 50},    // engagement = 100
		{Rkey: "medium", ReplyCount: 10, RepostCount: 15},  // engagement = 25
	}

	sorted := sortByEngagement(posts)

	// Should be descending by (replies + reposts)
	assert.Equal(t, "high", sorted[0].Rkey)
	assert.Equal(t, "medium", sorted[1].Rkey)
	assert.Equal(t, "low", sorted[2].Rkey)

	// Original should be unchanged (immutable)
	assert.Equal(t, "low", posts[0].Rkey)
}

// ============================================
// Filtering/Grouping Tests
// ============================================

func TestGetFrontPageGroups(t *testing.T) {
	groups := StoryGroups{
		"sg1": {ID: "sg1", SectionID: "front-page", Role: "headline", Priority: 1},
		"sg2": {ID: "sg2", SectionID: "front-page", Role: "", Priority: 2},
		"sg3": {ID: "sg3", SectionID: "front-page", Role: "opinion", Priority: 1},
		"sg4": {ID: "sg4", SectionID: "tech", Role: "", Priority: 1}, // Different section
	}

	result := getFrontPageGroups(groups)

	// Should have headline
	require.NotNil(t, result.Headline)
	assert.Equal(t, "sg1", result.Headline.ID)

	// Should have 1 story (sg2), not sg4 which is in tech
	assert.Len(t, result.Stories, 1)
	assert.Equal(t, "sg2", result.Stories[0].ID)

	// Should have 1 opinion
	assert.Len(t, result.Opinions, 1)
	assert.Equal(t, "sg3", result.Opinions[0].ID)
}

func TestGetSectionGroups(t *testing.T) {
	groups := StoryGroups{
		"sg1": {ID: "sg1", SectionID: "tech", Role: "headline", Priority: 1},
		"sg2": {ID: "sg2", SectionID: "tech", Role: "", Priority: 3},
		"sg3": {ID: "sg3", SectionID: "tech", Role: "", Priority: 2},
		"sg4": {ID: "sg4", SectionID: "tech", Role: "opinion", Priority: 1},
		"sg5": {ID: "sg5", SectionID: "sports", Role: "", Priority: 1}, // Different section
	}

	result := getSectionGroups(groups, "tech")

	// Should have headline
	require.NotNil(t, result.Headline)
	assert.Equal(t, "sg1", result.Headline.ID)

	// Should have 2 stories, sorted by priority (ascending)
	assert.Len(t, result.Stories, 2)
	assert.Equal(t, "sg3", result.Stories[0].ID) // priority 2
	assert.Equal(t, "sg2", result.Stories[1].ID) // priority 3

	// Should have 1 opinion
	assert.Len(t, result.Opinions, 1)
	assert.Equal(t, "sg4", result.Opinions[0].ID)
}

func TestGetSectionGroups_SortsbyPriority(t *testing.T) {
	groups := StoryGroups{
		"sg1": {ID: "sg1", SectionID: "tech", Role: "", Priority: 5},
		"sg2": {ID: "sg2", SectionID: "tech", Role: "", Priority: 1},
		"sg3": {ID: "sg3", SectionID: "tech", Role: "", Priority: 3},
	}

	result := getSectionGroups(groups, "tech")

	// Should be sorted by priority ascending (1, 3, 5)
	assert.Len(t, result.Stories, 3)
	assert.Equal(t, 1, result.Stories[0].Priority)
	assert.Equal(t, 3, result.Stories[1].Priority)
	assert.Equal(t, 5, result.Stories[2].Priority)
}

func TestGetSectionPosts(t *testing.T) {
	cats := Categories{
		"tech": {
			Visible: []string{"rkey1", "rkey2"},
			Hidden:  []string{"rkey3"},
		},
	}
	postIndex := map[string]Post{
		"rkey1": {Rkey: "rkey1", Text: "Post 1"},
		"rkey2": {Rkey: "rkey2", Text: "Post 2"},
		"rkey3": {Rkey: "rkey3", Text: "Post 3 (hidden)"},
	}

	posts := getSectionPosts("tech", cats, postIndex)

	// Should only return visible posts
	assert.Len(t, posts, 2)
	rkeys := []string{posts[0].Rkey, posts[1].Rkey}
	assert.Contains(t, rkeys, "rkey1")
	assert.Contains(t, rkeys, "rkey2")
}

func TestGetSectionPosts_EmptySection(t *testing.T) {
	cats := Categories{}
	postIndex := map[string]Post{}

	posts := getSectionPosts("nonexistent", cats, postIndex)

	assert.Empty(t, posts)
}

// ============================================
// Truncation Tests
// ============================================

func TestTruncateStories_Basic(t *testing.T) {
	headline := &StoryGroup{ID: "headline", Role: "headline"}
	groups := &GroupedStories{
		Headline: headline,
		Stories: []*StoryGroup{
			{ID: "s1"}, {ID: "s2"}, {ID: "s3"}, {ID: "s4"}, {ID: "s5"},
		},
		Opinions: []*StoryGroup{
			{ID: "o1"}, {ID: "o2"},
		},
	}

	truncateStories(groups, 4) // Max 4: 1 headline + 2 stories + 1 opinion

	assert.NotNil(t, groups.Headline)
	assert.LessOrEqual(t, len(groups.Stories)+len(groups.Opinions)+1, 4)
}

func TestTruncateStories_PreservesHeadline(t *testing.T) {
	headline := &StoryGroup{ID: "headline", Role: "headline"}
	groups := &GroupedStories{
		Headline: headline,
		Stories:  []*StoryGroup{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}},
		Opinions: []*StoryGroup{{ID: "o1"}},
	}

	truncateStories(groups, 1) // Only room for headline

	assert.NotNil(t, groups.Headline)
	assert.Empty(t, groups.Stories)
	assert.Empty(t, groups.Opinions)
}

func TestTruncateStories_PrioritizesStoriesOverOpinions(t *testing.T) {
	groups := &GroupedStories{
		Headline: nil,
		Stories:  []*StoryGroup{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}},
		Opinions: []*StoryGroup{{ID: "o1"}, {ID: "o2"}, {ID: "o3"}},
	}

	truncateStories(groups, 4) // 4 slots, should keep more stories than opinions

	// Should have stories + 1 opinion = 4
	assert.GreaterOrEqual(t, len(groups.Stories), len(groups.Opinions))
}

func TestTruncateStories_KeepsAtLeastOneOpinion(t *testing.T) {
	groups := &GroupedStories{
		Headline: nil,
		Stories:  []*StoryGroup{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}},
		Opinions: []*StoryGroup{{ID: "o1"}, {ID: "o2"}},
	}

	truncateStories(groups, 3)

	// Should keep at least 1 opinion if any existed
	assert.GreaterOrEqual(t, len(groups.Opinions), 1)
}

func TestTruncateStories_NoLimitDoesNothing(t *testing.T) {
	groups := &GroupedStories{
		Stories:  []*StoryGroup{{ID: "s1"}, {ID: "s2"}},
		Opinions: []*StoryGroup{{ID: "o1"}},
	}

	truncateStories(groups, 0) // No limit

	assert.Len(t, groups.Stories, 2)
	assert.Len(t, groups.Opinions, 1)
}

func TestTruncateStories_WithinLimitDoesNothing(t *testing.T) {
	groups := &GroupedStories{
		Stories:  []*StoryGroup{{ID: "s1"}, {ID: "s2"}},
		Opinions: []*StoryGroup{{ID: "o1"}},
	}

	truncateStories(groups, 10) // Way above total

	assert.Len(t, groups.Stories, 2)
	assert.Len(t, groups.Opinions, 1)
}

// ============================================
// Headline Fallback Tests
// ============================================

func TestGetHeadline_UsesHeadlineFirst(t *testing.T) {
	group := &StoryGroup{
		Headline:      "Explicit Headline",
		DraftHeadline: "Draft",
	}
	post := Post{ExternalLink: &ExternalLink{Title: "Link Title"}}

	result := getHeadline(group, post)

	assert.Equal(t, "Explicit Headline", result)
}

func TestGetHeadline_FallsBackToDraftHeadline(t *testing.T) {
	group := &StoryGroup{
		Headline:      "",
		DraftHeadline: "Draft Headline",
	}
	post := Post{ExternalLink: &ExternalLink{Title: "Link Title"}}

	result := getHeadline(group, post)

	assert.Equal(t, "Draft Headline", result)
}

func TestGetHeadline_FallsBackToExternalLinkTitle(t *testing.T) {
	group := &StoryGroup{
		Headline:      "",
		DraftHeadline: "",
	}
	post := Post{ExternalLink: &ExternalLink{Title: "External Link Title"}}

	result := getHeadline(group, post)

	assert.Equal(t, "External Link Title", result)
}

func TestGetHeadline_FallsBackToUntitled(t *testing.T) {
	group := &StoryGroup{
		Headline:      "",
		DraftHeadline: "",
	}
	post := Post{} // No external link

	result := getHeadline(group, post)

	assert.Equal(t, "(Untitled Story)", result)
}

// ============================================
// Text Transformation Tests
// ============================================

func TestDecodeUnicodeEscapes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"basic escape", `Hello \u0041`, "Hello A"},
		{"multiple escapes", `\u0048\u0065\u006c\u006c\u006f`, "Hello"},
		{"mixed content", `Test \u003c value \u003e`, "Test < value >"},
		{"no escapes", "Plain text", "Plain text"},
		{"emoji", `\u2764`, "❤"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeUnicodeEscapes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"simple https", "https://example.com/path", "example.com"},
		{"with port", "https://example.com:8080/path", "example.com:8080"},
		{"subdomain", "https://blog.example.com/post", "blog.example.com"},
		{"invalid url returns empty host", "not a url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDomain(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{"no truncation needed", "Hello", 10, "Hello"},
		{"exact length", "Hello", 5, "Hello"},
		{"needs truncation", "Hello World", 8, "Hello..."},
		{"very short max", "Hello World", 4, "H..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.text, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================
// Smoke Tests (Non-Brittle)
// ============================================

func TestCompileDigest_ReturnsNonEmpty(t *testing.T) {
	posts := []Post{{Rkey: "test1", Text: "Test post", Author: Author{Handle: "test.bsky.social"}}}
	cats := Categories{"tech": {Visible: []string{"test1"}}}
	storyGroups := StoryGroups{
		"sg1": {ID: "sg1", SectionID: "tech", PrimaryRkey: "test1", Headline: "Test Story", Priority: 1},
	}
	newspaperConfig := NewspaperConfig{
		Sections: []NewspaperSection{{ID: "tech", Name: "Technology"}},
	}
	contentPicks := AllContentPicks{}
	config := Config{CreatedAt: time.Now()}

	result, err := CompileDigest(posts, cats, storyGroups, newspaperConfig, contentPicks, config)

	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestCompileDigestHTML_ReturnsNonEmpty(t *testing.T) {
	posts := []Post{{Rkey: "test1", Text: "Test post", Author: Author{Handle: "test.bsky.social"}}}
	cats := Categories{"tech": {Visible: []string{"test1"}}}
	storyGroups := StoryGroups{
		"sg1": {ID: "sg1", SectionID: "tech", PrimaryRkey: "test1", Headline: "Test Story", Priority: 1},
	}
	newspaperConfig := NewspaperConfig{
		Sections: []NewspaperSection{{ID: "tech", Name: "Technology"}},
	}
	contentPicks := AllContentPicks{}
	config := Config{CreatedAt: time.Now()}

	result, err := CompileDigestHTML(posts, cats, storyGroups, newspaperConfig, contentPicks, config)

	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestCompileDigest_HandlesEmptyInput(t *testing.T) {
	posts := []Post{}
	cats := Categories{}
	storyGroups := StoryGroups{}
	newspaperConfig := NewspaperConfig{Sections: []NewspaperSection{}}
	contentPicks := AllContentPicks{}
	config := Config{CreatedAt: time.Now()}

	result, err := CompileDigest(posts, cats, storyGroups, newspaperConfig, contentPicks, config)

	require.NoError(t, err)
	assert.NotEmpty(t, result) // Should still have header
}

func TestCompileDigestHTML_HandlesEmptyInput(t *testing.T) {
	posts := []Post{}
	cats := Categories{}
	storyGroups := StoryGroups{}
	newspaperConfig := NewspaperConfig{Sections: []NewspaperSection{}}
	contentPicks := AllContentPicks{}
	config := Config{CreatedAt: time.Now()}

	result, err := CompileDigestHTML(posts, cats, storyGroups, newspaperConfig, contentPicks, config)

	require.NoError(t, err)
	assert.NotEmpty(t, result) // Should still have HTML structure
}
