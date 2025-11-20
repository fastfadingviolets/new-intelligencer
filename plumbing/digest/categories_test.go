package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCategorizePosts_NewCategory(t *testing.T) {
	cats := Categories{}
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
	}

	err := CategorizePosts(cats, index, "new-category", []string{"rkey1", "rkey2"})
	require.NoError(t, err)

	assert.Contains(t, cats, "new-category")
	assert.Equal(t, []string{"rkey1", "rkey2"}, cats["new-category"])
}

func TestCategorizePosts_ExistingCategory(t *testing.T) {
	cats := Categories{
		"existing": {"rkey1"},
	}
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
		"rkey3": 2,
	}

	err := CategorizePosts(cats, index, "existing", []string{"rkey2", "rkey3"})
	require.NoError(t, err)

	assert.Equal(t, []string{"rkey1", "rkey2", "rkey3"}, cats["existing"])
}

func TestCategorizePosts_MovesFromOldCategory(t *testing.T) {
	cats := Categories{
		"category-a": {"rkey1", "rkey2"},
		"category-b": {"rkey3"},
	}
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
		"rkey3": 2,
	}

	// Move rkey2 from category-a to category-b
	err := CategorizePosts(cats, index, "category-b", []string{"rkey2"})
	require.NoError(t, err)

	// Verify rkey2 removed from category-a
	assert.Equal(t, []string{"rkey1"}, cats["category-a"])

	// Verify rkey2 added to category-b
	assert.Equal(t, []string{"rkey3", "rkey2"}, cats["category-b"])
}

func TestCategorizePosts_Idempotent(t *testing.T) {
	cats := Categories{
		"test": {"rkey1"},
	}
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
	}

	// First categorization
	err := CategorizePosts(cats, index, "test", []string{"rkey2"})
	require.NoError(t, err)
	firstResult := cats["test"]

	// Second categorization (same operation)
	err = CategorizePosts(cats, index, "test", []string{"rkey2"})
	require.NoError(t, err)
	secondResult := cats["test"]

	// Should be the same
	assert.Equal(t, firstResult, secondResult)
	assert.Equal(t, []string{"rkey1", "rkey2"}, cats["test"])
}

func TestCategorizePosts_InvalidRkey(t *testing.T) {
	cats := Categories{}
	index := PostsIndex{
		"rkey1": 0,
	}

	// Try to categorize non-existent post
	err := CategorizePosts(cats, index, "test", []string{"nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestCategorizePosts_MultipleInvalidRkeys(t *testing.T) {
	cats := Categories{}
	index := PostsIndex{
		"rkey1": 0,
	}

	// Try to categorize mix of valid and invalid
	err := CategorizePosts(cats, index, "test", []string{"rkey1", "invalid1", "invalid2"})
	assert.Error(t, err)
}

func TestMergeCategories_Basic(t *testing.T) {
	cats := Categories{
		"source": {"rkey1", "rkey2"},
		"target": {"rkey3"},
	}

	err := MergeCategories(cats, "source", "target")
	require.NoError(t, err)

	// Source should be removed
	assert.NotContains(t, cats, "source")

	// Target should have all posts
	assert.Contains(t, cats, "target")
	assert.ElementsMatch(t, []string{"rkey3", "rkey1", "rkey2"}, cats["target"])
}

func TestMergeCategories_EmptySource(t *testing.T) {
	cats := Categories{
		"empty":  {},
		"target": {"rkey1"},
	}

	err := MergeCategories(cats, "empty", "target")
	require.NoError(t, err)

	assert.NotContains(t, cats, "empty")
	assert.Equal(t, []string{"rkey1"}, cats["target"])
}

func TestMergeCategories_NonExistentSource(t *testing.T) {
	cats := Categories{
		"target": {"rkey1"},
	}

	// Merging non-existent source should be safe (no-op)
	err := MergeCategories(cats, "nonexistent", "target")
	require.NoError(t, err)

	assert.Equal(t, []string{"rkey1"}, cats["target"])
}

func TestMergeCategories_Idempotent(t *testing.T) {
	cats := Categories{
		"source": {"rkey1"},
		"target": {"rkey2"},
	}

	// First merge
	err := MergeCategories(cats, "source", "target")
	require.NoError(t, err)
	firstState := make(Categories)
	for k, v := range cats {
		firstState[k] = append([]string{}, v...)
	}

	// Second merge (source already gone)
	err = MergeCategories(cats, "source", "target")
	require.NoError(t, err)

	// Should be same as after first merge
	assert.Equal(t, firstState, cats)
}

func TestGetCategoryPosts_Success(t *testing.T) {
	posts := []Post{
		{
			Rkey:      "rkey1",
			URI:       "at://did:plc:a/app.bsky.feed.post/rkey1",
			CID:       "cid1",
			Text:      "Post 1",
			Author:    Author{DID: "did:plc:a", Handle: "alice.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
		{
			Rkey:      "rkey2",
			URI:       "at://did:plc:b/app.bsky.feed.post/rkey2",
			CID:       "cid2",
			Text:      "Post 2",
			Author:    Author{DID: "did:plc:b", Handle: "bob.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
		{
			Rkey:      "rkey3",
			URI:       "at://did:plc:c/app.bsky.feed.post/rkey3",
			CID:       "cid3",
			Text:      "Post 3",
			Author:    Author{DID: "did:plc:c", Handle: "charlie.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
	}

	index := BuildIndex(posts)
	cats := Categories{
		"test-category": {"rkey1", "rkey3"},
	}

	displayPosts, err := GetCategoryPosts(cats, posts, index, "test-category")
	require.NoError(t, err)

	require.Len(t, displayPosts, 2)
	assert.Equal(t, "rkey1", displayPosts[0].Rkey)
	assert.Equal(t, "Post 1", displayPosts[0].Text)
	assert.Equal(t, "alice.bsky.social", displayPosts[0].Author.Handle)

	assert.Equal(t, "rkey3", displayPosts[1].Rkey)
	assert.Equal(t, "Post 3", displayPosts[1].Text)
}

func TestGetCategoryPosts_NonExistentCategory(t *testing.T) {
	posts := []Post{}
	index := PostsIndex{}
	cats := Categories{}

	displayPosts, err := GetCategoryPosts(cats, posts, index, "nonexistent")
	require.NoError(t, err) // Should not error, just return empty
	assert.Empty(t, displayPosts)
}

func TestGetCategoryPosts_EmptyCategory(t *testing.T) {
	posts := []Post{}
	index := PostsIndex{}
	cats := Categories{
		"empty": {},
	}

	displayPosts, err := GetCategoryPosts(cats, posts, index, "empty")
	require.NoError(t, err)
	assert.Empty(t, displayPosts)
}

func TestListCategoriesWithCounts(t *testing.T) {
	cats := Categories{
		"ai-discussions": {"rkey1", "rkey2", "rkey3"},
		"tech-news":      {"rkey4", "rkey5"},
		"meta-bsky":      {"rkey6"},
		"empty":          {},
	}

	counts := ListCategoriesWithCounts(cats)

	assert.Equal(t, 4, len(counts))
	assert.Equal(t, 3, counts["ai-discussions"])
	assert.Equal(t, 2, counts["tech-news"])
	assert.Equal(t, 1, counts["meta-bsky"])
	assert.Equal(t, 0, counts["empty"])
}

func TestListCategoriesWithCounts_Empty(t *testing.T) {
	cats := Categories{}

	counts := ListCategoriesWithCounts(cats)

	assert.Empty(t, counts)
}

func TestGetUncategorizedPosts(t *testing.T) {
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
		"rkey3": 2,
		"rkey4": 3,
		"rkey5": 4,
	}

	cats := Categories{
		"category-a": {"rkey1", "rkey3"},
		"category-b": {"rkey2"},
	}

	uncategorized := GetUncategorizedPosts(cats, index)

	// rkey4 and rkey5 are not in any category
	assert.Len(t, uncategorized, 2)
	assert.Contains(t, uncategorized, "rkey4")
	assert.Contains(t, uncategorized, "rkey5")
}

func TestGetUncategorizedPosts_AllCategorized(t *testing.T) {
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
	}

	cats := Categories{
		"category-a": {"rkey1", "rkey2"},
	}

	uncategorized := GetUncategorizedPosts(cats, index)

	assert.Empty(t, uncategorized)
}

func TestGetUncategorizedPosts_NoneCategorized(t *testing.T) {
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
		"rkey3": 2,
	}

	cats := Categories{}

	uncategorized := GetUncategorizedPosts(cats, index)

	assert.Len(t, uncategorized, 3)
	assert.Contains(t, uncategorized, "rkey1")
	assert.Contains(t, uncategorized, "rkey2")
	assert.Contains(t, uncategorized, "rkey3")
}

func TestCategorizePosts_RemovesFromMultipleCategories(t *testing.T) {
	// Edge case: post somehow in multiple categories (shouldn't happen, but be safe)
	cats := Categories{
		"cat-a": {"rkey1", "rkey2"},
		"cat-b": {"rkey2", "rkey3"}, // rkey2 in both (invalid state)
		"cat-c": {"rkey4"},
	}
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
		"rkey3": 2,
		"rkey4": 3,
	}

	// Move rkey2 to cat-c
	err := CategorizePosts(cats, index, "cat-c", []string{"rkey2"})
	require.NoError(t, err)

	// rkey2 should be removed from both cat-a and cat-b
	assert.NotContains(t, cats["cat-a"], "rkey2")
	assert.NotContains(t, cats["cat-b"], "rkey2")
	assert.Contains(t, cats["cat-c"], "rkey2")
}
