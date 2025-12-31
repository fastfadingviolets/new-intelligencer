package main

// FormatForDisplay converts posts from storage format to display format
// Removes unnecessary fields (URI, CID, IndexedAt, DIDs) to reduce token usage
func FormatForDisplay(posts []Post) []DisplayPost {
	if len(posts) == 0 {
		return []DisplayPost{}
	}

	display := make([]DisplayPost, len(posts))

	for i, post := range posts {
		dp := DisplayPost{
			Rkey: post.Rkey,
			Text: post.Text,
			Author: DisplayAuthor{
				Handle:      post.Author.Handle,
				DisplayName: post.Author.DisplayName,
			},
			CreatedAt: post.CreatedAt,
		}

		// Convert repost metadata if present
		if post.Repost != nil {
			dp.Repost = &DisplayRepost{
				ByHandle: post.Repost.ByHandle,
				At:       post.Repost.At,
			}
		}

		// Convert reply metadata if present
		if post.ReplyTo != nil {
			dp.ReplyTo = &DisplayReplyTo{
				AuthorHandle: post.ReplyTo.AuthorHandle,
			}
		}

		// Copy images if present
		if len(post.Images) > 0 {
			dp.Images = make([]Image, len(post.Images))
			copy(dp.Images, post.Images)
		}

		// Copy external link if present
		dp.ExternalLink = post.ExternalLink

		// Copy engagement metrics
		dp.LikeCount = post.LikeCount
		dp.ReplyCount = post.ReplyCount
		dp.RepostCount = post.RepostCount
		dp.QuoteCount = post.QuoteCount

		display[i] = dp
	}

	return display
}
