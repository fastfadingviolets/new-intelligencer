package main

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// formatDigestDate formats a date as "DD MMM YYYY" (e.g., "20 November 2025")
func formatDigestDate(t time.Time) string {
	return t.Format("02 January 2006")
}

// slugify converts a string to a URL-friendly slug
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "&", "and")
	// Remove any remaining non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	s = reg.ReplaceAllString(s, "")
	return s
}

// postURL generates the bsky.app URL for a post
func postURL(post Post) string {
	return fmt.Sprintf("https://bsky.app/profile/%s/post/%s", post.Author.Handle, post.Rkey)
}

// formatPostTime formats post timestamp for human reading
func formatPostTime(t time.Time) string {
	return t.Format("Jan 02, 3:04pm")
}

// getHeadline returns the effective headline for a story
// Fallback: Headline > DraftHeadline > Embed Title > "(Untitled Story)"
func getHeadline(group *StoryGroup, post Post) string {
	if group.Headline != "" {
		return group.Headline
	}
	if group.DraftHeadline != "" {
		return group.DraftHeadline
	}
	// Try embed title from primary post
	if post.ExternalLink != nil && post.ExternalLink.Title != "" {
		return post.ExternalLink.Title
	}
	return "(Untitled Story)"
}

// Helper functions for CompileNewspaper

// GroupedStories holds categorized story groups for a section or front page
type GroupedStories struct {
	Headline *StoryGroup
	Stories  []*StoryGroup // Regular stories, ordered by priority
	Opinions []*StoryGroup
}

func getFrontPageGroups(groups StoryGroups) GroupedStories {
	result := GroupedStories{}

	for id := range groups {
		group := groups[id]
		// Front page stories are those in the "front-page" section
		if group.SectionID != "front-page" {
			continue
		}
		groupCopy := group // Make a copy to take address of
		switch group.Role {
		case "headline":
			result.Headline = &groupCopy
		case "opinion":
			result.Opinions = append(result.Opinions, &groupCopy)
		default:
			result.Stories = append(result.Stories, &groupCopy)
		}
	}

	// Sort by priority (lower = higher priority)
	sort.Slice(result.Stories, func(i, j int) bool {
		return result.Stories[i].Priority < result.Stories[j].Priority
	})
	sort.Slice(result.Opinions, func(i, j int) bool {
		return result.Opinions[i].Priority < result.Opinions[j].Priority
	})

	return result
}

func getSectionGroups(groups StoryGroups, sectionID string) GroupedStories {
	result := GroupedStories{}

	for id := range groups {
		group := groups[id]
		if group.SectionID != sectionID {
			continue
		}
		groupCopy := group // Make a copy to take address of
		switch group.Role {
		case "headline":
			result.Headline = &groupCopy
		case "opinion":
			result.Opinions = append(result.Opinions, &groupCopy)
		default:
			result.Stories = append(result.Stories, &groupCopy)
		}
	}

	// Sort by priority (lower = higher priority)
	sort.Slice(result.Stories, func(i, j int) bool {
		return result.Stories[i].Priority < result.Stories[j].Priority
	})
	sort.Slice(result.Opinions, func(i, j int) bool {
		return result.Opinions[i].Priority < result.Opinions[j].Priority
	})

	return result
}

// truncateStories limits the total number of stories (headline + stories + opinions)
// to maxStories. Headline is always kept, stories and opinions are truncated proportionally.
func truncateStories(groups *GroupedStories, maxStories int) {
	if maxStories <= 0 {
		return // No limit
	}

	// Headline counts as 1 if present
	headlineCount := 0
	if groups.Headline != nil {
		headlineCount = 1
	}

	// Calculate how many stories + opinions we can keep
	remaining := maxStories - headlineCount
	if remaining <= 0 {
		// Only room for headline
		groups.Stories = nil
		groups.Opinions = nil
		return
	}

	totalNonHeadline := len(groups.Stories) + len(groups.Opinions)
	if totalNonHeadline <= remaining {
		return // Already within limit
	}

	// Truncate: prioritize stories over opinions, but keep at least 1 opinion if there are any
	if len(groups.Opinions) > 0 && remaining > 0 {
		// Keep at least 1 opinion
		storySlots := remaining - 1
		if len(groups.Stories) <= storySlots {
			// All stories fit, truncate opinions
			opinionSlots := remaining - len(groups.Stories)
			if len(groups.Opinions) > opinionSlots {
				groups.Opinions = groups.Opinions[:opinionSlots]
			}
		} else {
			// Truncate stories, keep 1 opinion
			groups.Stories = groups.Stories[:storySlots]
			groups.Opinions = groups.Opinions[:1]
		}
	} else {
		// No opinions, just truncate stories
		if len(groups.Stories) > remaining {
			groups.Stories = groups.Stories[:remaining]
		}
	}
}

func getSectionPosts(sectionID string, cats Categories, postIndex map[string]Post) []Post {
	var posts []Post

	// Categories.json keys ARE section IDs now (no more section-assignments indirection)
	if catData, ok := cats[sectionID]; ok {
		for _, rkey := range catData.Visible {
			if post, ok := postIndex[rkey]; ok {
				posts = append(posts, post)
			}
		}
	}

	return posts
}

func sortByLikes(posts []Post) []Post {
	sorted := make([]Post, len(posts))
	copy(sorted, posts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].LikeCount > sorted[j].LikeCount
	})
	return sorted
}

func sortByEngagement(posts []Post) []Post {
	sorted := make([]Post, len(posts))
	copy(sorted, posts)
	sort.Slice(sorted, func(i, j int) bool {
		engI := sorted[i].ReplyCount + sorted[i].RepostCount
		engJ := sorted[j].ReplyCount + sorted[j].RepostCount
		return engI > engJ
	})
	return sorted
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// decodeUnicodeEscapes decodes \uXXXX sequences to actual characters
var unicodeEscapeRegex = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

func decodeUnicodeEscapes(text string) string {
	return unicodeEscapeRegex.ReplaceAllStringFunc(text, func(match string) string {
		// Parse the hex value (skip the \u prefix)
		code, err := strconv.ParseInt(match[2:], 16, 32)
		if err != nil {
			return match // Keep original if parsing fails
		}
		return string(rune(code))
	})
}

// escapeHTML escapes special HTML characters (after decoding unicode escapes)
func escapeHTML(text string) string {
	text = decodeUnicodeEscapes(text)
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// extractDomain extracts the domain from a URL for display
func extractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Host
}

// CompileDigestHTML generates an HTML newspaper-style digest with interleaved content.
// Uses Go templates for rendering.
func CompileDigestHTML(
	posts []Post,
	cats Categories,
	storyGroups StoryGroups,
	newspaperConfig NewspaperConfig,
	contentPicks AllContentPicks,
	config Config,
) (string, error) {
	// Build template data and render
	data := BuildDigestData(posts, cats, storyGroups, newspaperConfig, contentPicks, config)
	return RenderDigest(data)
}


