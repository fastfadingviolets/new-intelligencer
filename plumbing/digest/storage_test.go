package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractRkey_FromURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{
			name:     "valid post URI",
			uri:      "at://did:plc:xyz123/app.bsky.feed.post/3lbkj2x3abcd",
			expected: "3lbkj2x3abcd",
		},
		{
			name:     "different DID",
			uri:      "at://did:plc:abc456/app.bsky.feed.post/4mcxk3y4defg",
			expected: "4mcxk3y4defg",
		},
		{
			name:     "long rkey",
			uri:      "at://did:plc:test/app.bsky.feed.post/verylongrkey1234567890",
			expected: "verylongrkey1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractRkey(tt.uri)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadPosts_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	postsFile := filepath.Join(tmpDir, "posts.json")

	// Create empty array file
	err := os.WriteFile(postsFile, []byte("[]"), 0644)
	require.NoError(t, err)

	posts, err := LoadPosts(postsFile)
	require.NoError(t, err)
	assert.Empty(t, posts)
}

func TestLoadPosts_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	postsFile := filepath.Join(tmpDir, "nonexistent.json")

	posts, err := LoadPosts(postsFile)
	require.NoError(t, err) // Should return empty array, not error
	assert.Empty(t, posts)
}

func TestLoadPosts_ValidData(t *testing.T) {
	tmpDir := t.TempDir()
	postsFile := filepath.Join(tmpDir, "posts.json")

	// Create test posts
	testPosts := []Post{
		{
			Rkey:      "3lbkj2x3abcd",
			URI:       "at://did:plc:xyz/app.bsky.feed.post/3lbkj2x3abcd",
			CID:       "bafyreiabc123",
			Text:      "Test post about AI",
			Author:    Author{DID: "did:plc:xyz", Handle: "alice.bsky.social", DisplayName: "Alice"},
			CreatedAt: time.Date(2025, 11, 20, 8, 15, 0, 0, time.UTC),
			IndexedAt: time.Date(2025, 11, 20, 8, 15, 30, 0, time.UTC),
		},
		{
			Rkey:      "4mcxk3y4defg",
			URI:       "at://did:plc:abc/app.bsky.feed.post/4mcxk3y4defg",
			CID:       "bafyreidef456",
			Text:      "Another test post",
			Author:    Author{DID: "did:plc:abc", Handle: "bob.bsky.social", DisplayName: "Bob"},
			CreatedAt: time.Date(2025, 11, 20, 9, 22, 0, 0, time.UTC),
			IndexedAt: time.Date(2025, 11, 20, 9, 22, 15, 0, time.UTC),
			Repost: &Repost{
				ByDID:    "did:plc:def",
				ByHandle: "charlie.bsky.social",
				At:       time.Date(2025, 11, 20, 9, 45, 0, 0, time.UTC),
			},
		},
	}

	// Write test data
	data, err := json.MarshalIndent(testPosts, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(postsFile, data, 0644)
	require.NoError(t, err)

	// Load and verify
	posts, err := LoadPosts(postsFile)
	require.NoError(t, err)
	require.Len(t, posts, 2)

	assert.Equal(t, "3lbkj2x3abcd", posts[0].Rkey)
	assert.Equal(t, "Test post about AI", posts[0].Text)
	assert.Equal(t, "alice.bsky.social", posts[0].Author.Handle)
	assert.Nil(t, posts[0].Repost)

	assert.Equal(t, "4mcxk3y4defg", posts[1].Rkey)
	assert.NotNil(t, posts[1].Repost)
	assert.Equal(t, "charlie.bsky.social", posts[1].Repost.ByHandle)
}

func TestLoadPosts_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	postsFile := filepath.Join(tmpDir, "posts.json")

	// Write invalid JSON
	err := os.WriteFile(postsFile, []byte("{invalid json}"), 0644)
	require.NoError(t, err)

	_, err = LoadPosts(postsFile)
	assert.Error(t, err)
}

func TestSavePosts_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	postsFile := filepath.Join(tmpDir, "posts.json")

	testPosts := []Post{
		{
			Rkey:      "test123",
			URI:       "at://did:plc:test/app.bsky.feed.post/test123",
			CID:       "bafyrei123",
			Text:      "Test",
			Author:    Author{DID: "did:plc:test", Handle: "test.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
	}

	err := SavePosts(postsFile, testPosts)
	require.NoError(t, err)

	// Verify file exists and contains correct data
	assert.FileExists(t, postsFile)

	loaded, err := LoadPosts(postsFile)
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "test123", loaded[0].Rkey)
	assert.Equal(t, "Test", loaded[0].Text)
}

func TestLoadCategories_EmptyAndMissing(t *testing.T) {
	tmpDir := t.TempDir()

	// Test missing file
	missingFile := filepath.Join(tmpDir, "missing.json")
	cats, err := LoadCategories(missingFile)
	require.NoError(t, err)
	assert.Empty(t, cats)

	// Test empty object
	emptyFile := filepath.Join(tmpDir, "empty.json")
	err = os.WriteFile(emptyFile, []byte("{}"), 0644)
	require.NoError(t, err)

	cats, err = LoadCategories(emptyFile)
	require.NoError(t, err)
	assert.Empty(t, cats)
}

func TestLoadCategories_ValidData(t *testing.T) {
	tmpDir := t.TempDir()
	catsFile := filepath.Join(tmpDir, "categories.json")

	// Write old format (map[string][]string) to test backward compatibility
	oldFormat := map[string][]string{
		"ai-discussions": {"3lbkj2x3abcd", "6oezm5a6klmn"},
		"tech-news":      {"4mcxk3y4defg", "7pfan6b7opqr"},
	}

	data, err := json.MarshalIndent(oldFormat, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(catsFile, data, 0644)
	require.NoError(t, err)

	cats, err := LoadCategories(catsFile)
	require.NoError(t, err)
	assert.Len(t, cats, 2)
	// Should be migrated to new format
	assert.Equal(t, []string{"3lbkj2x3abcd", "6oezm5a6klmn"}, cats["ai-discussions"].Visible)
	assert.Equal(t, []string{"4mcxk3y4defg", "7pfan6b7opqr"}, cats["tech-news"].Visible)
}

func TestSaveCategories(t *testing.T) {
	tmpDir := t.TempDir()
	catsFile := filepath.Join(tmpDir, "categories.json")

	testCats := Categories{
		"test-category": CategoryData{Visible: []string{"rkey1", "rkey2"}},
	}

	err := SaveCategories(catsFile, testCats)
	require.NoError(t, err)

	loaded, err := LoadCategories(catsFile)
	require.NoError(t, err)
	assert.Equal(t, testCats, loaded)
}

func TestBuildIndex(t *testing.T) {
	posts := []Post{
		{Rkey: "abc123"},
		{Rkey: "def456"},
		{Rkey: "ghi789"},
	}

	index := BuildIndex(posts)

	assert.Len(t, index, 3)
	assert.Equal(t, 0, index["abc123"])
	assert.Equal(t, 1, index["def456"])
	assert.Equal(t, 2, index["ghi789"])
}

func TestLoadIndex(t *testing.T) {
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "posts-index.json")

	testIndex := PostsIndex{
		"abc123": 0,
		"def456": 1,
	}

	data, err := json.MarshalIndent(testIndex, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(indexFile, data, 0644)
	require.NoError(t, err)

	index, err := LoadIndex(indexFile)
	require.NoError(t, err)
	assert.Equal(t, testIndex, index)
}

func TestSaveIndex(t *testing.T) {
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "posts-index.json")

	testIndex := PostsIndex{
		"test123": 0,
	}

	err := SaveIndex(indexFile, testIndex)
	require.NoError(t, err)

	loaded, err := LoadIndex(indexFile)
	require.NoError(t, err)
	assert.Equal(t, testIndex, loaded)
}
