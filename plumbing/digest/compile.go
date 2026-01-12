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

// CompileDigest generates a newspaper-style digest with front page and sections
func CompileDigest(
	posts []Post,
	cats Categories,
	storyGroups StoryGroups,
	newspaperConfig NewspaperConfig,
	contentPicks AllContentPicks,
	config Config,
) (string, error) {
	var md strings.Builder

	// Build post index for quick lookup
	postIndex := make(map[string]Post)
	for _, post := range posts {
		postIndex[post.Rkey] = post
	}

	// Track referenced posts for references section
	referenced := make(map[string]bool)

	// Title
	md.WriteString(fmt.Sprintf("# The Daily Digest - %s\n\n", formatDigestDate(config.CreatedAt)))
	md.WriteString("---\n\n")

	// Front Page
	md.WriteString("## FRONT PAGE\n\n")

	// Front page headline
	frontPageGroups := getFrontPageGroups(storyGroups)
	// Apply truncation based on max_stories from newspaper.json
	for _, section := range newspaperConfig.Sections {
		if section.ID == "front-page" && section.MaxStories > 0 {
			truncateStories(&frontPageGroups, section.MaxStories)
			break
		}
	}
	if frontPageGroups.Headline != nil {
		headline := frontPageGroups.Headline
		post := postIndex[headline.PrimaryRkey]
		md.WriteString(fmt.Sprintf("### TOP STORY: [%s](%s)\n\n", getHeadline(headline, post), postURL(post)))
		if headline.Summary != "" {
			md.WriteString(headline.Summary + "\n\n")
		}
		if headline.ArticleURL != "" {
			md.WriteString(fmt.Sprintf("Read more: %s\n\n", headline.ArticleURL))
		}
		for _, rkey := range headline.PostRkeys {
			referenced[rkey] = true
		}
	}

	// Front page stories
	if len(frontPageGroups.Stories) > 0 {
		md.WriteString("**Stories:**\n")
		for _, group := range frontPageGroups.Stories {
			post := postIndex[group.PrimaryRkey]
			md.WriteString(fmt.Sprintf("- [%s](%s)\n", getHeadline(group, post), postURL(post)))
			for _, rkey := range group.PostRkeys {
				referenced[rkey] = true
			}
		}
		md.WriteString("\n")
	}

	// Front page opinions
	if len(frontPageGroups.Opinions) > 0 {
		md.WriteString("**Opinion:**\n")
		for _, group := range frontPageGroups.Opinions {
			post := postIndex[group.PrimaryRkey]
			snippet := truncateText(post.Text, 100)
			md.WriteString(fmt.Sprintf("- [Opinion: %s](%s)\n  > %s - @%s\n\n", getHeadline(group, post), postURL(post), snippet, post.Author.Handle))
			for _, rkey := range group.PostRkeys {
				referenced[rkey] = true
			}
		}
	}

	md.WriteString("---\n\n")

	// Sections
	for _, section := range newspaperConfig.Sections {
		// Skip front-page section (rendered above)
		if section.ID == "front-page" {
			continue
		}

		if section.Type == "news" {
			md.WriteString(fmt.Sprintf("## %s (News)\n\n", section.Name))

			// Get story groups for this section
			sectionGroups := getSectionGroups(storyGroups, section.ID)
			// Apply truncation based on max_stories
			if section.MaxStories > 0 {
				truncateStories(&sectionGroups, section.MaxStories)
			}

			// Headline
			if sectionGroups.Headline != nil {
				headline := sectionGroups.Headline
				post := postIndex[headline.PrimaryRkey]
				md.WriteString(fmt.Sprintf("### Headline: [%s](%s)\n\n", getHeadline(headline, post), postURL(post)))
				if headline.Summary != "" {
					md.WriteString(headline.Summary + "\n\n")
				}
				for _, rkey := range headline.PostRkeys {
					referenced[rkey] = true
				}
			}

			// Stories
			if len(sectionGroups.Stories) > 0 {
				md.WriteString("**Stories:**\n")
				for i, group := range sectionGroups.Stories {
					post := postIndex[group.PrimaryRkey]
					md.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, getHeadline(group, post), postURL(post)))
					for _, rkey := range group.PostRkeys {
						referenced[rkey] = true
					}
				}
				md.WriteString("\n")
			}

			// Opinions
			if len(sectionGroups.Opinions) > 0 {
				md.WriteString("**Opinion:**\n")
				for _, group := range sectionGroups.Opinions {
					post := postIndex[group.PrimaryRkey]
					snippet := truncateText(post.Text, 80)
					md.WriteString(fmt.Sprintf("- [Opinion: %s](%s)\n  > %s - @%s\n\n", getHeadline(group, post), postURL(post), snippet, post.Author.Handle))
					for _, rkey := range group.PostRkeys {
						referenced[rkey] = true
					}
				}
			}

		} else if section.Type == "content" {
			md.WriteString(fmt.Sprintf("## %s (Content)\n\n", section.Name))

			// Get all posts in this section (categories.json keys ARE section IDs now)
			sectionPosts := getSectionPosts(section.ID, cats, postIndex)

			// Most Popular (by likes)
			if len(sectionPosts) > 0 {
				popular := sortByLikes(sectionPosts)
				md.WriteString("**Most Popular:**\n")
				for i, post := range popular[:min(section.MaxStories, len(popular))] {
					snippet := truncateText(post.Text, 60)
					md.WriteString(fmt.Sprintf("%d. @%s: \"%s\" (%d likes) [%s]\n",
						i+1, post.Author.Handle, snippet, post.LikeCount, post.Rkey))
					referenced[post.Rkey] = true
				}
				md.WriteString("\n")
			}

			// Most Engaging (by replies + reposts)
			if len(sectionPosts) > 0 {
				engaging := sortByEngagement(sectionPosts)
				md.WriteString("**Most Engaging:**\n")
				for i, post := range engaging[:min(section.MaxStories, len(engaging))] {
					snippet := truncateText(post.Text, 60)
					engagement := post.ReplyCount + post.RepostCount
					md.WriteString(fmt.Sprintf("%d. @%s: \"%s\" (%d interactions) [%s]\n",
						i+1, post.Author.Handle, snippet, engagement, post.Rkey))
					referenced[post.Rkey] = true
				}
				md.WriteString("\n")
			}

			// Sui Generis
			if picks, ok := contentPicks[section.ID]; ok && len(picks.SuiGeneris) > 0 {
				md.WriteString("**Sui Generis:**\n")
				suiGenerisLimit := min(section.MaxStories, len(picks.SuiGeneris))
				for _, rkey := range picks.SuiGeneris[:suiGenerisLimit] {
					if post, ok := postIndex[rkey]; ok {
						snippet := truncateText(post.Text, 80)
						md.WriteString(fmt.Sprintf("- @%s: \"%s\" [%s]\n", post.Author.Handle, snippet, post.Rkey))
						referenced[post.Rkey] = true
					}
				}
				md.WriteString("\n")
			}

			// Full Feed (collapsible)
			if len(sectionPosts) > 0 {
				md.WriteString(fmt.Sprintf("<details>\n<summary>Full Feed (%d posts)</summary>\n\n", len(sectionPosts)))
				for _, post := range sectionPosts {
					timeStr := formatPostTime(post.CreatedAt)
					snippet := truncateText(post.Text, 120)
					md.WriteString(fmt.Sprintf("- [%s] @%s (%s): \"%s\"\n", post.Rkey, post.Author.Handle, timeStr, snippet))
				}
				md.WriteString("\n</details>\n\n")
			}
		}

		md.WriteString("---\n\n")
	}

	// References section
	md.WriteString("## References\n\n")

	var refPosts []Post
	for rkey := range referenced {
		if post, ok := postIndex[rkey]; ok {
			refPosts = append(refPosts, post)
		}
	}

	// Sort by creation time
	sort.Slice(refPosts, func(i, j int) bool {
		return refPosts[i].CreatedAt.Before(refPosts[j].CreatedAt)
	})

	for _, post := range refPosts {
		timeStr := formatPostTime(post.CreatedAt)
		md.WriteString(fmt.Sprintf("[%s] %s (%s)", post.Rkey, post.Author.Handle, timeStr))

		if post.Repost != nil {
			md.WriteString(fmt.Sprintf(" [reposted by %s]", post.Repost.ByHandle))
		}
		md.WriteString("\n")

		md.WriteString(fmt.Sprintf("\"%s\"\n", post.Text))
		md.WriteString(fmt.Sprintf("%s\n\n", postURL(post)))
	}

	return md.String(), nil
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
	text = strings.ReplaceAll(text, "\"", "&quot;")
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


