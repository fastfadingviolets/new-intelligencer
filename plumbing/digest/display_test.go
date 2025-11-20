package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatForDisplay_RemovesURICID(t *testing.T) {
	posts := []Post{
		{
			Rkey:      "3lbkj2x3abcd",
			URI:       "at://did:plc:xyz/app.bsky.feed.post/3lbkj2x3abcd",
			CID:       "bafyreiabc123",
			Text:      "Test post",
			Author:    Author{DID: "did:plc:xyz", Handle: "alice.bsky.social", DisplayName: "Alice"},
			CreatedAt: time.Date(2025, 11, 20, 8, 15, 0, 0, time.UTC),
			IndexedAt: time.Date(2025, 11, 20, 8, 15, 30, 0, time.UTC),
		},
	}

	display := FormatForDisplay(posts)

	require.Len(t, display, 1)
	// Verify URI, CID, IndexedAt, and DID are NOT in the display format
	// (We can't directly check they're absent, but we verify what IS present)
	assert.Equal(t, "3lbkj2x3abcd", display[0].Rkey)
	assert.Equal(t, "Test post", display[0].Text)
	assert.Equal(t, "alice.bsky.social", display[0].Author.Handle)
	assert.Equal(t, "Alice", display[0].Author.DisplayName)
}

func TestFormatForDisplay_KeepsEssentials(t *testing.T) {
	createdAt := time.Date(2025, 11, 20, 8, 15, 0, 0, time.UTC)
	posts := []Post{
		{
			Rkey:      "test123",
			URI:       "at://did:plc:xyz/app.bsky.feed.post/test123",
			CID:       "bafyrei123",
			Text:      "Essential data test",
			Author:    Author{DID: "did:plc:xyz", Handle: "test.bsky.social", DisplayName: "Tester"},
			CreatedAt: createdAt,
			IndexedAt: time.Now(),
		},
	}

	display := FormatForDisplay(posts)

	require.Len(t, display, 1)
	d := display[0]

	// Verify all essential fields are present
	assert.Equal(t, "test123", d.Rkey)
	assert.Equal(t, "Essential data test", d.Text)
	assert.Equal(t, "test.bsky.social", d.Author.Handle)
	assert.Equal(t, "Tester", d.Author.DisplayName)
	assert.Equal(t, createdAt, d.CreatedAt)
}

func TestFormatForDisplay_PreservesRepostMetadata(t *testing.T) {
	repostAt := time.Date(2025, 11, 20, 9, 45, 0, 0, time.UTC)
	posts := []Post{
		{
			Rkey:      "repost123",
			URI:       "at://did:plc:abc/app.bsky.feed.post/repost123",
			CID:       "bafyreiabc",
			Text:      "Original post text",
			Author:    Author{DID: "did:plc:abc", Handle: "bob.bsky.social", DisplayName: "Bob"},
			CreatedAt: time.Date(2025, 11, 20, 9, 22, 0, 0, time.UTC),
			IndexedAt: time.Now(),
			Repost: &Repost{
				ByDID:    "did:plc:def",
				ByHandle: "charlie.bsky.social",
				At:       repostAt,
			},
		},
	}

	display := FormatForDisplay(posts)

	require.Len(t, display, 1)
	d := display[0]

	// Verify repost metadata is preserved (except ByDID)
	require.NotNil(t, d.Repost)
	assert.Equal(t, "charlie.bsky.social", d.Repost.ByHandle)
	assert.Equal(t, repostAt, d.Repost.At)
}

func TestFormatForDisplay_PreservesReplyMetadata(t *testing.T) {
	posts := []Post{
		{
			Rkey:      "reply123",
			URI:       "at://did:plc:xyz/app.bsky.feed.post/reply123",
			CID:       "bafyrei123",
			Text:      "@someone I agree",
			Author:    Author{DID: "did:plc:xyz", Handle: "alice.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
			ReplyTo: &ReplyTo{
				URI:          "at://did:plc:parent/app.bsky.feed.post/parent123",
				AuthorHandle: "someone.bsky.social",
			},
		},
	}

	display := FormatForDisplay(posts)

	require.Len(t, display, 1)
	d := display[0]

	// Verify reply metadata is preserved (except URI)
	require.NotNil(t, d.ReplyTo)
	assert.Equal(t, "someone.bsky.social", d.ReplyTo.AuthorHandle)
}

func TestFormatForDisplay_PreservesImages(t *testing.T) {
	posts := []Post{
		{
			Rkey:      "image123",
			URI:       "at://did:plc:xyz/app.bsky.feed.post/image123",
			CID:       "bafyrei123",
			Text:      "Post with images",
			Author:    Author{DID: "did:plc:xyz", Handle: "alice.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
			Images: []Image{
				{URL: "https://cdn.bsky.app/img/1.jpg", Alt: "First image"},
				{URL: "https://cdn.bsky.app/img/2.jpg", Alt: "Second image"},
			},
		},
	}

	display := FormatForDisplay(posts)

	require.Len(t, display, 1)
	d := display[0]

	// Verify images are preserved
	require.Len(t, d.Images, 2)
	assert.Equal(t, "https://cdn.bsky.app/img/1.jpg", d.Images[0].URL)
	assert.Equal(t, "First image", d.Images[0].Alt)
	assert.Equal(t, "https://cdn.bsky.app/img/2.jpg", d.Images[1].URL)
	assert.Equal(t, "Second image", d.Images[1].Alt)
}

func TestFormatForDisplay_OmitsAbsentFields(t *testing.T) {
	posts := []Post{
		{
			Rkey:      "simple123",
			URI:       "at://did:plc:xyz/app.bsky.feed.post/simple123",
			CID:       "bafyrei123",
			Text:      "Simple post with no extras",
			Author:    Author{DID: "did:plc:xyz", Handle: "alice.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
			// No Repost, ReplyTo, or Images
		},
	}

	display := FormatForDisplay(posts)

	require.Len(t, display, 1)
	d := display[0]

	// Verify optional fields are nil/empty when not present
	assert.Nil(t, d.Repost)
	assert.Nil(t, d.ReplyTo)
	assert.Nil(t, d.Images) // or Empty, depending on implementation
}

func TestFormatForDisplay_EmptyArray(t *testing.T) {
	posts := []Post{}

	display := FormatForDisplay(posts)

	assert.Empty(t, display)
}

func TestFormatForDisplay_MultiplePostsMixedContent(t *testing.T) {
	posts := []Post{
		{
			Rkey:      "post1",
			URI:       "at://did:plc:a/app.bsky.feed.post/post1",
			CID:       "bafyrei1",
			Text:      "Simple post",
			Author:    Author{DID: "did:plc:a", Handle: "alice.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
		{
			Rkey:      "post2",
			URI:       "at://did:plc:b/app.bsky.feed.post/post2",
			CID:       "bafyrei2",
			Text:      "Post with repost",
			Author:    Author{DID: "did:plc:b", Handle: "bob.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
			Repost: &Repost{
				ByDID:    "did:plc:c",
				ByHandle: "charlie.bsky.social",
				At:       time.Now(),
			},
		},
		{
			Rkey:      "post3",
			URI:       "at://did:plc:d/app.bsky.feed.post/post3",
			CID:       "bafyrei3",
			Text:      "Reply post",
			Author:    Author{DID: "did:plc:d", Handle: "dave.bsky.social"},
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
			ReplyTo: &ReplyTo{
				URI:          "at://did:plc:parent/app.bsky.feed.post/parent",
				AuthorHandle: "parent.bsky.social",
			},
		},
	}

	display := FormatForDisplay(posts)

	require.Len(t, display, 3)

	// First post: simple
	assert.Equal(t, "post1", display[0].Rkey)
	assert.Nil(t, display[0].Repost)
	assert.Nil(t, display[0].ReplyTo)

	// Second post: has repost
	assert.Equal(t, "post2", display[1].Rkey)
	assert.NotNil(t, display[1].Repost)
	assert.Equal(t, "charlie.bsky.social", display[1].Repost.ByHandle)

	// Third post: has reply
	assert.Equal(t, "post3", display[2].Rkey)
	assert.NotNil(t, display[2].ReplyTo)
	assert.Equal(t, "parent.bsky.social", display[2].ReplyTo.AuthorHandle)
}

func TestFormatForDisplay_DisplayNameOptional(t *testing.T) {
	posts := []Post{
		{
			Rkey:      "noname123",
			URI:       "at://did:plc:xyz/app.bsky.feed.post/noname123",
			CID:       "bafyrei123",
			Text:      "Post without display name",
			Author:    Author{DID: "did:plc:xyz", Handle: "user.bsky.social"}, // No DisplayName
			CreatedAt: time.Now(),
			IndexedAt: time.Now(),
		},
	}

	display := FormatForDisplay(posts)

	require.Len(t, display, 1)
	assert.Equal(t, "user.bsky.social", display[0].Author.Handle)
	assert.Empty(t, display[0].Author.DisplayName)
}
