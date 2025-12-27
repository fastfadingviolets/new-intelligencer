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
	assert.Equal(t, []string{"rkey1", "rkey2"}, cats["new-category"].Visible)
}

func TestCategorizePosts_ExistingCategory(t *testing.T) {
	cats := Categories{
		"existing": CategoryData{Visible: []string{"rkey1"}},
	}
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
		"rkey3": 2,
	}

	err := CategorizePosts(cats, index, "existing", []string{"rkey2", "rkey3"})
	require.NoError(t, err)

	assert.Equal(t, []string{"rkey1", "rkey2", "rkey3"}, cats["existing"].Visible)
}

func TestCategorizePosts_MovesFromOldCategory(t *testing.T) {
	cats := Categories{
		"category-a": CategoryData{Visible: []string{"rkey1", "rkey2"}},
		"category-b": CategoryData{Visible: []string{"rkey3"}},
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
	assert.Equal(t, []string{"rkey1"}, cats["category-a"].Visible)

	// Verify rkey2 added to category-b
	assert.Equal(t, []string{"rkey3", "rkey2"}, cats["category-b"].Visible)
}

func TestCategorizePosts_Idempotent(t *testing.T) {
	cats := Categories{
		"test": CategoryData{Visible: []string{"rkey1"}},
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
	assert.Equal(t, []string{"rkey1", "rkey2"}, cats["test"].Visible)
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
		"source": CategoryData{Visible: []string{"rkey1", "rkey2"}},
		"target": CategoryData{Visible: []string{"rkey3"}},
	}

	err := MergeCategories(cats, "source", "target")
	require.NoError(t, err)

	// Source should be removed
	assert.NotContains(t, cats, "source")

	// Target should have all posts
	assert.Contains(t, cats, "target")
	assert.ElementsMatch(t, []string{"rkey3", "rkey1", "rkey2"}, cats["target"].Visible)
}

func TestMergeCategories_EmptySource(t *testing.T) {
	cats := Categories{
		"empty":  CategoryData{Visible: []string{}},
		"target": CategoryData{Visible: []string{"rkey1"}},
	}

	err := MergeCategories(cats, "empty", "target")
	require.NoError(t, err)

	assert.NotContains(t, cats, "empty")
	assert.Equal(t, []string{"rkey1"}, cats["target"].Visible)
}

func TestMergeCategories_NonExistentSource(t *testing.T) {
	cats := Categories{
		"target": CategoryData{Visible: []string{"rkey1"}},
	}

	// Merging non-existent source should be safe (no-op)
	err := MergeCategories(cats, "nonexistent", "target")
	require.NoError(t, err)

	assert.Equal(t, []string{"rkey1"}, cats["target"].Visible)
}

func TestMergeCategories_Idempotent(t *testing.T) {
	cats := Categories{
		"source": CategoryData{Visible: []string{"rkey1"}},
		"target": CategoryData{Visible: []string{"rkey2"}},
	}

	// First merge
	err := MergeCategories(cats, "source", "target")
	require.NoError(t, err)

	// Save first state (deep copy)
	firstVisible := append([]string{}, cats["target"].Visible...)
	firstHidden := cats["target"].Hidden
	firstReason := cats["target"].HiddenReason

	// Second merge (source already gone)
	err = MergeCategories(cats, "source", "target")
	require.NoError(t, err)

	// Should be same as after first merge
	assert.ElementsMatch(t, firstVisible, cats["target"].Visible)
	assert.Equal(t, firstHidden, cats["target"].Hidden)
	assert.Equal(t, firstReason, cats["target"].HiddenReason)
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
		"test-category": CategoryData{Visible: []string{"rkey1", "rkey3"}},
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
		"empty": CategoryData{Visible: []string{}},
	}

	displayPosts, err := GetCategoryPosts(cats, posts, index, "empty")
	require.NoError(t, err)
	assert.Empty(t, displayPosts)
}

func TestListCategoriesWithCounts(t *testing.T) {
	cats := Categories{
		"ai-discussions": CategoryData{Visible: []string{"rkey1", "rkey2", "rkey3"}},
		"tech-news":      CategoryData{Visible: []string{"rkey4", "rkey5"}},
		"meta-bsky":      CategoryData{Visible: []string{"rkey6"}},
		"empty":          CategoryData{Visible: []string{}},
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
		"category-a": CategoryData{Visible: []string{"rkey1", "rkey3"}},
		"category-b": CategoryData{Visible: []string{"rkey2"}},
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
		"category-a": CategoryData{Visible: []string{"rkey1", "rkey2"}},
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
		"cat-a": CategoryData{Visible: []string{"rkey1", "rkey2"}},
		"cat-b": CategoryData{Visible: []string{"rkey2", "rkey3"}}, // rkey2 in both (invalid state)
		"cat-c": CategoryData{Visible: []string{"rkey4"}},
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
	assert.NotContains(t, cats["cat-a"].Visible, "rkey2")
	assert.NotContains(t, cats["cat-b"].Visible, "rkey2")
	assert.Contains(t, cats["cat-c"].Visible, "rkey2")
}

func TestHideCategory_Basic(t *testing.T) {
	cats := Categories{
		"category-a": CategoryData{Visible: []string{"rkey1", "rkey2"}},
		"category-b": CategoryData{Visible: []string{"rkey3"}},
	}

	count, err := HideCategory(cats, "category-a")
	require.NoError(t, err)

	assert.Equal(t, 2, count)
	assert.Contains(t, cats, "category-a") // Category still exists
	assert.True(t, cats["category-a"].IsHidden)
	assert.Contains(t, cats, "category-b") // Other categories unaffected
	assert.False(t, cats["category-b"].IsHidden)
}

func TestHideCategory_NonExistent(t *testing.T) {
	cats := Categories{
		"category-a": CategoryData{Visible: []string{"rkey1"}},
	}

	count, err := HideCategory(cats, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Equal(t, 0, count)

	// Original category still exists and visible
	assert.Contains(t, cats, "category-a")
	assert.False(t, cats["category-a"].IsHidden)
}

func TestHideCategory_AlreadyHidden(t *testing.T) {
	cats := Categories{
		"category-a": CategoryData{Visible: []string{"rkey1"}, IsHidden: true},
	}

	count, err := HideCategory(cats, "category-a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already hidden")
	assert.Equal(t, 0, count)
}

func TestHideCategory_WithHiddenPosts(t *testing.T) {
	cats := Categories{
		"category-a": CategoryData{
			Visible: []string{"rkey1", "rkey2"},
			Hidden:  []string{"rkey3", "rkey4"},
		},
	}

	count, err := HideCategory(cats, "category-a")
	require.NoError(t, err)

	assert.Equal(t, 4, count) // Counts both visible and hidden posts
	assert.True(t, cats["category-a"].IsHidden)
}

func TestUnhideCategory_Basic(t *testing.T) {
	cats := Categories{
		"category-a": CategoryData{Visible: []string{"rkey1"}, IsHidden: true},
		"category-b": CategoryData{Visible: []string{"rkey2"}},
	}

	err := UnhideCategory(cats, "category-a")
	require.NoError(t, err)

	assert.False(t, cats["category-a"].IsHidden)
	assert.Contains(t, cats, "category-a")
}

func TestUnhideCategory_NonExistent(t *testing.T) {
	cats := Categories{
		"category-a": CategoryData{Visible: []string{"rkey1"}},
	}

	err := UnhideCategory(cats, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUnhideCategory_NotHidden(t *testing.T) {
	cats := Categories{
		"category-a": CategoryData{Visible: []string{"rkey1"}},
	}

	err := UnhideCategory(cats, "category-a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not hidden")
}

func TestHidePosts_Basic(t *testing.T) {
	cats := Categories{
		"test-category": CategoryData{
			Visible: []string{"rkey1", "rkey2", "rkey3"},
			Hidden:  []string{},
		},
	}

	err := HidePosts(cats, "test-category", []string{"rkey2", "rkey3"}, "repetitive")
	require.NoError(t, err)

	catData := cats["test-category"]
	assert.Equal(t, []string{"rkey1"}, catData.Visible)
	assert.ElementsMatch(t, []string{"rkey2", "rkey3"}, catData.Hidden)
	assert.Equal(t, "repetitive", catData.HiddenReason)
}

func TestHidePosts_MultipleReasons(t *testing.T) {
	cats := Categories{
		"test": CategoryData{
			Visible:      []string{"rkey1", "rkey2", "rkey3"},
			Hidden:       []string{"rkey4"},
			HiddenReason: "low-quality",
		},
	}

	err := HidePosts(cats, "test", []string{"rkey2"}, "repetitive")
	require.NoError(t, err)

	catData := cats["test"]
	assert.Equal(t, []string{"rkey1", "rkey3"}, catData.Visible)
	assert.ElementsMatch(t, []string{"rkey4", "rkey2"}, catData.Hidden)
	assert.Equal(t, "low-quality; repetitive", catData.HiddenReason)
}

func TestHidePosts_SameReason(t *testing.T) {
	cats := Categories{
		"test": CategoryData{
			Visible:      []string{"rkey1", "rkey2"},
			Hidden:       []string{"rkey3"},
			HiddenReason: "repetitive",
		},
	}

	err := HidePosts(cats, "test", []string{"rkey2"}, "repetitive")
	require.NoError(t, err)

	catData := cats["test"]
	// Same reason shouldn't be appended
	assert.Equal(t, "repetitive", catData.HiddenReason)
}

func TestHidePosts_NonExistentCategory(t *testing.T) {
	cats := Categories{
		"existing": CategoryData{Visible: []string{"rkey1"}},
	}

	err := HidePosts(cats, "nonexistent", []string{"rkey1"}, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHidePosts_NonExistentPost(t *testing.T) {
	cats := Categories{
		"test": CategoryData{Visible: []string{"rkey1", "rkey2"}},
	}

	err := HidePosts(cats, "test", []string{"rkey_nonexistent"}, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Category should be unchanged
	assert.Equal(t, []string{"rkey1", "rkey2"}, cats["test"].Visible)
}

func TestHidePosts_AllPosts(t *testing.T) {
	cats := Categories{
		"test": CategoryData{
			Visible: []string{"rkey1", "rkey2"},
		},
	}

	err := HidePosts(cats, "test", []string{"rkey1", "rkey2"}, "all boring")
	require.NoError(t, err)

	catData := cats["test"]
	assert.Empty(t, catData.Visible)
	assert.ElementsMatch(t, []string{"rkey1", "rkey2"}, catData.Hidden)
	assert.Equal(t, "all boring", catData.HiddenReason)
}

func TestHidePosts_EmptyReason(t *testing.T) {
	cats := Categories{
		"test": CategoryData{
			Visible: []string{"rkey1", "rkey2"},
		},
	}

	err := HidePosts(cats, "test", []string{"rkey1"}, "")
	require.NoError(t, err)

	catData := cats["test"]
	assert.Equal(t, []string{"rkey2"}, catData.Visible)
	assert.Equal(t, []string{"rkey1"}, catData.Hidden)
	assert.Equal(t, "", catData.HiddenReason)
}

func TestGetCategoryPosts_FiltersHidden(t *testing.T) {
	posts := []Post{
		{
			Rkey:      "rkey1",
			URI:       "at://did:plc:a/app.bsky.feed.post/rkey1",
			CID:       "cid1",
			Text:      "Visible post 1",
			Author:    Author{DID: "did:plc:a", Handle: "alice.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
		{
			Rkey:      "rkey2",
			URI:       "at://did:plc:b/app.bsky.feed.post/rkey2",
			CID:       "cid2",
			Text:      "Hidden post",
			Author:    Author{DID: "did:plc:b", Handle: "bob.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
		{
			Rkey:      "rkey3",
			URI:       "at://did:plc:c/app.bsky.feed.post/rkey3",
			CID:       "cid3",
			Text:      "Visible post 2",
			Author:    Author{DID: "did:plc:c", Handle: "charlie.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
	}

	index := BuildIndex(posts)
	cats := Categories{
		"test-category": CategoryData{
			Visible: []string{"rkey1", "rkey3"},
			Hidden:  []string{"rkey2"},
		},
	}

	displayPosts, err := GetCategoryPosts(cats, posts, index, "test-category")
	require.NoError(t, err)

	// Should only return visible posts
	require.Len(t, displayPosts, 2)
	assert.Equal(t, "rkey1", displayPosts[0].Rkey)
	assert.Equal(t, "Visible post 1", displayPosts[0].Text)
	assert.Equal(t, "rkey3", displayPosts[1].Rkey)
	assert.Equal(t, "Visible post 2", displayPosts[1].Text)
}

func TestGetUncategorizedPosts_CountsHiddenAsCategorized(t *testing.T) {
	index := PostsIndex{
		"rkey1": 0,
		"rkey2": 1,
		"rkey3": 2,
		"rkey4": 3,
	}

	cats := Categories{
		"test": CategoryData{
			Visible: []string{"rkey1"},
			Hidden:  []string{"rkey2"},
		},
	}

	uncategorized := GetUncategorizedPosts(cats, index)

	// rkey2 is hidden but still categorized, so only rkey3 and rkey4 are uncategorized
	assert.Len(t, uncategorized, 2)
	assert.Contains(t, uncategorized, "rkey3")
	assert.Contains(t, uncategorized, "rkey4")
}
