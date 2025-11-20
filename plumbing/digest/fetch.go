package main

import (
	"context"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
)

// FetchConfig holds configuration for fetching posts
type FetchConfig struct {
	Handle   string
	Password string
	PDSHost  string
	Since    time.Time
	Limit    int
}

// FetchResult holds the results of a fetch operation
type FetchResult struct {
	Posts []Post
	Index PostsIndex
	Total int
}

// Authenticate creates an authenticated XRPC client
func Authenticate(handle, password, pdsHost string) (*xrpc.Client, error) {
	ctx := context.Background()

	client := &xrpc.Client{
		Host: pdsHost,
	}

	authResp, err := atproto.ServerCreateSession(ctx, client, &atproto.ServerCreateSession_Input{
		Identifier: handle,
		Password:   password,
	})
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	client.Auth = &xrpc.AuthInfo{
		AccessJwt:  authResp.AccessJwt,
		RefreshJwt: authResp.RefreshJwt,
		Handle:     authResp.Handle,
		Did:        authResp.Did,
	}

	return client, nil
}

// FetchPosts fetches posts from Bluesky timeline
func FetchPosts(client *xrpc.Client, since time.Time, limit int) (*FetchResult, error) {
	ctx := context.Background()
	var allPosts []Post
	cursor := ""

fetchLoop:
	for {
		// Check if we've hit the limit
		if limit > 0 && len(allPosts) >= limit {
			break
		}

		// Fetch one page
		timelineResp, err := bsky.FeedGetTimeline(ctx, client, "reverse-chronological", cursor, 50)
		if err != nil {
			return nil, fmt.Errorf("fetching timeline: %w", err)
		}

		if len(timelineResp.Feed) == 0 {
			break
		}

		// Update cursor for next iteration
		if timelineResp.Cursor != nil && *timelineResp.Cursor != "" {
			cursor = *timelineResp.Cursor
		} else {
			cursor = "" // No more pages
		}

		// Convert and filter posts
		for _, item := range timelineResp.Feed {
			post := ConvertAPIPost(item)

			// Filter by time - use repost timestamp if available, otherwise created_at
			timeToCheck := post.CreatedAt
			if post.Repost != nil {
				timeToCheck = post.Repost.At
			}

			if timeToCheck.Before(since) {
				continue // Skip old posts
			}

			allPosts = append(allPosts, post)

			// Check limit after adding
			if limit > 0 && len(allPosts) >= limit {
				break fetchLoop
			}
		}

		// No more pages
		if cursor == "" {
			break
		}
	}

	// Build index
	index := BuildIndex(allPosts)

	return &FetchResult{
		Posts: allPosts,
		Index: index,
		Total: len(allPosts),
	}, nil
}

// ConvertAPIPost converts a Bluesky API response item to our Post format
func ConvertAPIPost(item *bsky.FeedDefs_FeedViewPost) Post {
	post := Post{
		URI: item.Post.Uri,
		CID: item.Post.Cid,
		Author: Author{
			DID:         item.Post.Author.Did,
			Handle:      item.Post.Author.Handle,
			DisplayName: getStringPtr(item.Post.Author.DisplayName),
		},
	}

	// Extract rkey from URI
	post.Rkey = ExtractRkey(post.URI)

	// Extract post content
	if postView, ok := item.Post.Record.Val.(*bsky.FeedPost); ok {
		post.Text = postView.Text
		post.CreatedAt, _ = time.Parse(time.RFC3339, postView.CreatedAt)

		// Extract reply reference
		if postView.Reply != nil {
			post.ReplyTo = &ReplyTo{
				URI:          postView.Reply.Parent.Uri,
				AuthorHandle: "", // Not available in reply ref, would need separate lookup
			}
		}

		// TODO: Extract images - need to determine correct field names in indigo
		// if postView.Embed != nil && postView.Embed.EmbedImages != nil {
		// 	images := postView.Embed.EmbedImages.Images
		// 	post.Images = make([]Image, 0, len(images))
		// 	for _, img := range images {
		// 		post.Images = append(post.Images, Image{
		// 			URL: img.???,  // Field name TBD
		// 			Alt: img.Alt,
		// 		})
		// 	}
		// }
	}

	// Set indexed time to now
	post.IndexedAt = time.Now()

	// Check if this is a repost and extract repost metadata
	if item.Reason != nil && item.Reason.FeedDefs_ReasonRepost != nil {
		repostReason := item.Reason.FeedDefs_ReasonRepost
		repostTime, _ := time.Parse(time.RFC3339, repostReason.IndexedAt)

		post.Repost = &Repost{
			ByDID:    repostReason.By.Did,
			ByHandle: repostReason.By.Handle,
			At:       repostTime,
		}
	}

	return post
}

// Helper: get string from pointer (empty string if nil)
func getStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
