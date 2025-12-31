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

	}

	// Extract images and external links from embed view data
	if item.Post.Embed != nil {
		// Direct image embeds
		if item.Post.Embed.EmbedImages_View != nil {
			for _, img := range item.Post.Embed.EmbedImages_View.Images {
				post.Images = append(post.Images, Image{
					URL: img.Fullsize,
					Alt: img.Alt,
				})
			}
		}

		// Record with media (quote posts with images)
		if item.Post.Embed.EmbedRecordWithMedia_View != nil {
			media := item.Post.Embed.EmbedRecordWithMedia_View.Media
			if media != nil && media.EmbedImages_View != nil {
				for _, img := range media.EmbedImages_View.Images {
					post.Images = append(post.Images, Image{
						URL: img.Fullsize,
						Alt: img.Alt,
					})
				}
			}
		}

		// External link embeds (articles, websites)
		if item.Post.Embed.EmbedExternal_View != nil {
			ext := item.Post.Embed.EmbedExternal_View.External
			post.ExternalLink = &ExternalLink{
				URL:         ext.Uri,
				Title:       ext.Title,
				Description: ext.Description,
				Thumb:       getStringPtr(ext.Thumb),
			}
		}
	}

	// Set indexed time to now
	post.IndexedAt = time.Now()

	// Extract engagement metrics
	if item.Post.LikeCount != nil {
		post.LikeCount = *item.Post.LikeCount
	}
	if item.Post.ReplyCount != nil {
		post.ReplyCount = *item.Post.ReplyCount
	}
	if item.Post.RepostCount != nil {
		post.RepostCount = *item.Post.RepostCount
	}
	if item.Post.QuoteCount != nil {
		post.QuoteCount = *item.Post.QuoteCount
	}

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
