package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Handle   string `envconfig:"BSKY_HANDLE" required:"true"`
	Password string `envconfig:"BSKY_PASSWORD" required:"true"`
	PDSHost  string `envconfig:"BSKY_PDS_HOST" default:"https://bsky.social"`
}

type Post struct {
	URI        string     `json:"uri"`
	CID        string     `json:"cid"`
	Author     Author     `json:"author"`
	Text       string     `json:"text"`
	CreatedAt  time.Time  `json:"created_at"`
	RepostedAt *time.Time `json:"reposted_at,omitempty"` // When this was reposted (if it's a repost)
	ReplyTo    *ReplyRef  `json:"reply_to,omitempty"`
	Images     []Image    `json:"images,omitempty"`
	RepostOf   *RepostRef `json:"repost_of,omitempty"`
}

type Author struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name,omitempty"`
}

type ReplyRef struct {
	URI    string `json:"uri"`
	Author Author `json:"author"`
}

type RepostRef struct {
	URI    string `json:"uri"`
	Author Author `json:"author"`
}

type Image struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

type Output struct {
	Posts      []Post    `json:"posts"`
	FetchedAt  time.Time `json:"fetched_at"`
	TimeRange  string    `json:"time_range"`
	TotalCount int       `json:"total_count"`
	Cursor     *string   `json:"cursor"` // Next cursor for pagination, null when done
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  fetch-feed    Fetch posts from your timeline\n")
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "fetch-feed":
		err = fetchFeed()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func fetchFeed() error {
	fs := flag.NewFlagSet("fetch-feed", flag.ExitOnError)
	since := fs.String("since", "", "Fetch posts since this time (RFC3339 format, default: 24h ago)")
	limit := fs.Int("limit", 100, "Maximum number of posts to fetch")
	cursorFlag := fs.String("cursor", "", "Pagination cursor from previous request")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	// Load config from environment
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Parse time range
	var sinceTime time.Time
	if *since != "" {
		var err error
		sinceTime, err = time.Parse(time.RFC3339, *since)
		if err != nil {
			return fmt.Errorf("parsing --since time: %w", err)
		}
	} else {
		sinceTime = time.Now().Add(-24 * time.Hour)
	}

	ctx := context.Background()

	// Create XRPC client
	client := &xrpc.Client{
		Host: cfg.PDSHost,
	}

	// Authenticate
	authResp, err := atproto.ServerCreateSession(ctx, client, &atproto.ServerCreateSession_Input{
		Identifier: cfg.Handle,
		Password:   cfg.Password,
	})
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	client.Auth = &xrpc.AuthInfo{
		AccessJwt:  authResp.AccessJwt,
		RefreshJwt: authResp.RefreshJwt,
		Handle:     authResp.Handle,
		Did:        authResp.Did,
	}

	// Fetch timeline
	var allPosts []Post
	cursor := *cursorFlag // Use cursor from flag, empty string if not provided

fetchLoop:
	for len(allPosts) < *limit {
		timelineResp, err := bsky.FeedGetTimeline(ctx, client, "reverse-chronological", cursor, 50)
		if err != nil {
			return fmt.Errorf("fetching timeline: %w", err)
		}

		if len(timelineResp.Feed) == 0 {
			break
		}

		// Save cursor from this response before processing posts
		if timelineResp.Cursor != nil && *timelineResp.Cursor != "" {
			cursor = *timelineResp.Cursor
		} else {
			cursor = "" // No more pages available
		}

		for _, item := range timelineResp.Feed {
			post := convertPost(item)

			// Filter by time - use repost timestamp if available, otherwise original post time
			timeToCheck := post.CreatedAt
			if post.RepostedAt != nil {
				timeToCheck = *post.RepostedAt
			}

			if timeToCheck.Before(sinceTime) {
				continue // Skip old posts but keep looking for newer ones
			}

			allPosts = append(allPosts, post)
			if len(allPosts) >= *limit {
				break fetchLoop
			}
		}

		// If we ran out of pages, stop
		if cursor == "" {
			break
		}
	}

	// Output JSON
	var outputCursor *string
	if cursor != "" {
		outputCursor = &cursor
	}

	output := Output{
		Posts:      allPosts,
		FetchedAt:  time.Now(),
		TimeRange:  fmt.Sprintf("since %s", sinceTime.Format(time.RFC3339)),
		TotalCount: len(allPosts),
		Cursor:     outputCursor,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	return nil
}

func convertPost(item *bsky.FeedDefs_FeedViewPost) Post {
	post := Post{}

	// Extract post record
	if postView, ok := item.Post.Record.Val.(*bsky.FeedPost); ok {
		post.Text = postView.Text
		post.CreatedAt, _ = time.Parse(time.RFC3339, postView.CreatedAt)

		// Extract reply reference
		if postView.Reply != nil {
			post.ReplyTo = &ReplyRef{
				URI: postView.Reply.Parent.Uri,
			}
		}
	}

	// Extract author info
	post.URI = item.Post.Uri
	post.CID = item.Post.Cid
	post.Author = Author{
		DID:         item.Post.Author.Did,
		Handle:      item.Post.Author.Handle,
		DisplayName: getStringPtr(item.Post.Author.DisplayName),
	}

	// Check if this is a repost and extract repost timestamp
	if item.Reason != nil && item.Reason.FeedDefs_ReasonRepost != nil {
		repostReason := item.Reason.FeedDefs_ReasonRepost
		if repostedTime, err := time.Parse(time.RFC3339, repostReason.IndexedAt); err == nil {
			post.RepostedAt = &repostedTime
		}
		post.RepostOf = &RepostRef{
			URI: item.Post.Uri,
			Author: Author{
				DID:    repostReason.By.Did,
				Handle: repostReason.By.Handle,
			},
		}
	}

	return post
}

func getStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
