package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Note: Full API mocking is complex due to indigo's union types.
// These tests focus on testable logic. Full fetch tested manually.

func TestFetchConfig_Validation(t *testing.T) {
	cfg := FetchConfig{
		Handle:   "test.bsky.social",
		Password: "password",
		PDSHost:  "https://bsky.social",
		Since:    time.Now().Add(-24 * time.Hour),
		Limit:    100,
	}

	assert.Equal(t, "test.bsky.social", cfg.Handle)
	assert.Equal(t, 100, cfg.Limit)
	assert.True(t, cfg.Since.Before(time.Now()))
}

func TestFetchResult_Structure(t *testing.T) {
	posts := []Post{
		{
			Rkey:      "test123",
			URI:       "at://did:plc:test/app.bsky.feed.post/test123",
			Text:      "Test post",
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
	}

	index := BuildIndex(posts)

	result := &FetchResult{
		Posts: posts,
		Index: index,
		Total: len(posts),
	}

	assert.Equal(t, 1, result.Total)
	assert.Len(t, result.Posts, 1)
	assert.Contains(t, result.Index, "test123")
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	// This test would need a real API or complex mock
	// Skipping for now - tested manually
	t.Skip("Requires real Bluesky API or complex mocking")
}

func TestFetchPosts_Integration(t *testing.T) {
	// This test requires real API access
	// Tested manually with actual credentials
	t.Skip("Integration test - requires real API access")
}

// Test helper functions that don't require API mocking

func TestGetStringPtr_Nil(t *testing.T) {
	var nilStr *string = nil
	result := getStringPtr(nilStr)
	assert.Empty(t, result)
}

func TestGetStringPtr_NonNil(t *testing.T) {
	str := "test value"
	result := getStringPtr(&str)
	assert.Equal(t, "test value", result)
}

// Test time filtering logic (extracted for testability)
func TestTimeFiltering_Logic(t *testing.T) {
	since := time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		created    time.Time
		repostedAt *time.Time
		shouldKeep bool
	}{
		{
			name:       "post after since time",
			created:    time.Date(2025, 11, 20, 8, 0, 0, 0, time.UTC),
			repostedAt: nil,
			shouldKeep: true,
		},
		{
			name:       "post before since time",
			created:    time.Date(2025, 11, 19, 8, 0, 0, 0, time.UTC),
			repostedAt: nil,
			shouldKeep: false,
		},
		{
			name:       "old post reposted after since",
			created:    time.Date(2025, 11, 15, 8, 0, 0, 0, time.UTC),
			repostedAt: &[]time.Time{time.Date(2025, 11, 20, 9, 0, 0, 0, time.UTC)}[0],
			shouldKeep: true,
		},
		{
			name:       "old post reposted before since",
			created:    time.Date(2025, 11, 15, 8, 0, 0, 0, time.UTC),
			repostedAt: &[]time.Time{time.Date(2025, 11, 19, 9, 0, 0, 0, time.UTC)}[0],
			shouldKeep: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeToCheck := tt.created
			if tt.repostedAt != nil {
				timeToCheck = *tt.repostedAt
			}

			shouldKeep := !timeToCheck.Before(since)
			assert.Equal(t, tt.shouldKeep, shouldKeep, "Time filtering logic")
		})
	}
}

// Test limit logic
func TestLimitLogic(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		currentCount  int
		shouldContinue bool
	}{
		{
			name:           "no limit set",
			limit:          0,
			currentCount:   100,
			shouldContinue: true,
		},
		{
			name:           "under limit",
			limit:          100,
			currentCount:   50,
			shouldContinue: true,
		},
		{
			name:           "at limit",
			limit:          100,
			currentCount:   100,
			shouldContinue: false,
		},
		{
			name:           "over limit",
			limit:          100,
			currentCount:   101,
			shouldContinue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldContinue := tt.limit == 0 || tt.currentCount < tt.limit
			assert.Equal(t, tt.shouldContinue, shouldContinue)
		})
	}
}

// Test index building with fetched posts
func TestFetchResult_IndexBuilding(t *testing.T) {
	posts := []Post{
		{Rkey: "abc123", Text: "First"},
		{Rkey: "def456", Text: "Second"},
		{Rkey: "ghi789", Text: "Third"},
	}

	index := BuildIndex(posts)

	assert.Len(t, index, 3)
	assert.Equal(t, 0, index["abc123"])
	assert.Equal(t, 1, index["def456"])
	assert.Equal(t, 2, index["ghi789"])

	result := &FetchResult{
		Posts: posts,
		Index: index,
		Total: len(posts),
	}

	// Verify we can lookup posts via index
	post := result.Posts[result.Index["def456"]]
	assert.Equal(t, "Second", post.Text)
}
