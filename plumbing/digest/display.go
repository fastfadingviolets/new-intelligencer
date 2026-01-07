package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// fetchAndEncodeImage fetches an image URL and returns base64-encoded data
func fetchAndEncodeImage(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Detect content type and create data URL
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg" // default
	}

	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data)), nil
}

// formatSinglePost converts a single Post to DisplayPost (helper function)
func formatSinglePost(post Post) DisplayPost {
	dp := DisplayPost{
		Rkey: post.Rkey,
		Text: post.Text,
		Author: DisplayAuthor{
			Handle:      post.Author.Handle,
			DisplayName: post.Author.DisplayName,
		},
		CreatedAt: post.CreatedAt,
	}

	if post.Repost != nil {
		dp.Repost = &DisplayRepost{
			ByHandle: post.Repost.ByHandle,
			At:       post.Repost.At,
		}
	}

	if post.ReplyTo != nil {
		dp.ReplyTo = &DisplayReplyTo{
			AuthorHandle: post.ReplyTo.AuthorHandle,
		}
	}

	if len(post.Images) > 0 {
		dp.Images = make([]Image, len(post.Images))
		copy(dp.Images, post.Images)
	}

	dp.ExternalLink = post.ExternalLink

	if post.Quote != nil {
		dp.Quote = &DisplayQuote{
			Rkey: post.Quote.Rkey,
			Text: post.Quote.Text,
			Author: DisplayAuthor{
				Handle:      post.Quote.Author.Handle,
				DisplayName: post.Quote.Author.DisplayName,
			},
			CreatedAt: post.Quote.CreatedAt,
		}
	}

	dp.LikeCount = post.LikeCount
	dp.ReplyCount = post.ReplyCount
	dp.RepostCount = post.RepostCount
	dp.QuoteCount = post.QuoteCount

	return dp
}

// FormatForDisplay converts posts from storage format to display format
// Removes unnecessary fields (URI, CID, IndexedAt, DIDs) to reduce token usage
func FormatForDisplay(posts []Post) []DisplayPost {
	if len(posts) == 0 {
		return []DisplayPost{}
	}

	display := make([]DisplayPost, len(posts))
	for i, post := range posts {
		display[i] = formatSinglePost(post)
	}
	return display
}

// FormatForDisplayWithThreads converts posts with thread replies nested
func FormatForDisplayWithThreads(roots []Post, wd *WorkspaceData) []DisplayPost {
	if len(roots) == 0 {
		return []DisplayPost{}
	}

	display := make([]DisplayPost, len(roots))
	for i, post := range roots {
		dp := formatSinglePost(post)

		// Add thread replies if any
		replyRkeys := wd.GetReplies(post.Rkey)
		if len(replyRkeys) > 0 {
			for _, replyRkey := range replyRkeys {
				if idx, ok := wd.Index[replyRkey]; ok && idx < len(wd.Posts) {
					replyPost := wd.Posts[idx]
					dp.ThreadReplies = append(dp.ThreadReplies, formatSinglePost(replyPost))
				}
			}
		}

		display[i] = dp
	}
	return display
}

// FetchImagesForDisplay fetches and encodes first image for each DisplayPost (concurrent)
// Modifies the slice in place.
func FetchImagesForDisplay(display []DisplayPost) {
	if len(display) == 0 {
		return
	}

	var wg sync.WaitGroup
	for i := range display {
		if len(display[i].Images) > 0 {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				imageData, err := fetchAndEncodeImage(display[idx].Images[0].URL)
				if err == nil {
					display[idx].ImageData = imageData
				}
				// Silently ignore errors - image just won't be included
			}(i)
		}
	}
	wg.Wait()
}
