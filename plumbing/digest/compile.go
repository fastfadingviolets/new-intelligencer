package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// CompileDigest generates the final markdown digest from posts, categories, and summaries
func CompileDigest(posts []Post, cats Categories, sums Summaries, config Config) (string, error) {
	var md strings.Builder

	// Title with date
	md.WriteString(fmt.Sprintf("# Bluesky Digest - %s\n\n", formatDigestDate(config.CreatedAt)))

	// Get sorted category names
	catNames := make([]string, 0, len(cats))
	for name := range cats {
		if len(cats[name]) > 0 { // Only include non-empty categories
			catNames = append(catNames, name)
		}
	}
	sort.Strings(catNames)

	// Build index for quick post lookup
	postIndex := make(map[string]Post)
	for _, post := range posts {
		postIndex[post.Rkey] = post
	}

	// Track which posts are referenced (for references section)
	referenced := make(map[string]bool)

	// Category sections
	for _, catName := range catNames {
		postCount := len(cats[catName])
		pluralS := "s"
		if postCount == 1 {
			pluralS = ""
		}

		md.WriteString(fmt.Sprintf("## %s (%d post%s)\n\n", catName, postCount, pluralS))

		// Add summary if available
		if summary, ok := sums[catName]; ok && summary != "" {
			md.WriteString(summary)
			md.WriteString("\n\n")

			// Track referenced posts
			for _, rkey := range cats[catName] {
				if strings.Contains(summary, "["+rkey+"]") {
					referenced[rkey] = true
				}
			}
		} else {
			// No summary available
			md.WriteString("(No summary available)\n\n")
		}
	}

	// Separator before references
	md.WriteString("---\n\n")

	// References section
	md.WriteString("## References\n\n")

	// Get all referenced posts in order they appear in categories
	var refPosts []Post
	seenRefs := make(map[string]bool)

	for _, catName := range catNames {
		for _, rkey := range cats[catName] {
			if referenced[rkey] && !seenRefs[rkey] {
				if post, ok := postIndex[rkey]; ok {
					refPosts = append(refPosts, post)
					seenRefs[rkey] = true
				}
			}
		}
	}

	// Write reference entries
	for _, post := range refPosts {
		// Reference header: [rkey] handle (date time)
		timeStr := formatPostTime(post.CreatedAt)
		md.WriteString(fmt.Sprintf("[%s] %s (%s)", post.Rkey, post.Author.Handle, timeStr))

		// Add repost indicator if applicable
		if post.Repost != nil {
			md.WriteString(fmt.Sprintf(" [reposted by %s]", post.Repost.ByHandle))
		}

		md.WriteString("\n")

		// Quote the post text
		md.WriteString(fmt.Sprintf("\"%s\"\n", post.Text))

		// URL to post
		md.WriteString(fmt.Sprintf("%s\n\n", postURL(post)))
	}

	return md.String(), nil
}

// formatDigestDate formats a date as "DD MMM YYYY" (e.g., "20 November 2025")
func formatDigestDate(t time.Time) string {
	return t.Format("02 January 2006")
}

// postURL generates the bsky.app URL for a post
func postURL(post Post) string {
	return fmt.Sprintf("https://bsky.app/profile/%s/post/%s", post.Author.Handle, post.Rkey)
}

// formatPostTime formats post timestamp for human reading
func formatPostTime(t time.Time) string {
	return t.Format("Jan 02, 3:04pm")
}

// escapeMarkdown escapes special markdown characters
// Note: For quoted text in references, we generally don't need heavy escaping
// as it's in a quote context. But this function is here for future use.
func escapeMarkdown(text string) string {
	// Basic escaping - can be enhanced if needed
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "*", "\\*")
	text = strings.ReplaceAll(text, "_", "\\_")
	text = strings.ReplaceAll(text, "[", "\\[")
	text = strings.ReplaceAll(text, "]", "\\]")
	return text
}
