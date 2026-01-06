package main

import (
	"fmt"
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
		md.WriteString(fmt.Sprintf("### TOP STORY: [%s](%s)\n\n", headline.Headline, postURL(post)))
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

	// Front page featured
	if len(frontPageGroups.Featured) > 0 {
		md.WriteString("**Featured Stories:**\n")
		for _, group := range frontPageGroups.Featured {
			post := postIndex[group.PrimaryRkey]
			md.WriteString(fmt.Sprintf("- [%s](%s)\n", group.Headline, postURL(post)))
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
			md.WriteString(fmt.Sprintf("- [Opinion: %s](%s)\n  > %s - @%s\n\n", group.Headline, postURL(post), snippet, post.Author.Handle))
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
				md.WriteString(fmt.Sprintf("### Headline: [%s](%s)\n\n", headline.Headline, postURL(post)))
				if headline.Summary != "" {
					md.WriteString(headline.Summary + "\n\n")
				}
				for _, rkey := range headline.PostRkeys {
					referenced[rkey] = true
				}
			}

			// Featured
			if len(sectionGroups.Featured) > 0 {
				md.WriteString("**Featured:**\n")
				for i, group := range sectionGroups.Featured {
					post := postIndex[group.PrimaryRkey]
					md.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, group.Headline, postURL(post)))
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
					md.WriteString(fmt.Sprintf("- [Opinion: %s](%s)\n  > %s - @%s\n\n", group.Headline, postURL(post), snippet, post.Author.Handle))
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
				for i, post := range popular[:min(3, len(popular))] {
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
				for i, post := range engaging[:min(3, len(engaging))] {
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
				for _, rkey := range picks.SuiGeneris {
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
	Featured []*StoryGroup
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
		case "featured":
			result.Featured = append(result.Featured, &groupCopy)
		case "opinion":
			result.Opinions = append(result.Opinions, &groupCopy)
		}
	}

	// Sort by priority (lower = higher priority)
	sort.Slice(result.Featured, func(i, j int) bool {
		return result.Featured[i].Priority < result.Featured[j].Priority
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
		case "featured":
			result.Featured = append(result.Featured, &groupCopy)
		case "opinion":
			result.Opinions = append(result.Opinions, &groupCopy)
		}
	}

	// Sort by priority (lower = higher priority)
	sort.Slice(result.Featured, func(i, j int) bool {
		return result.Featured[i].Priority < result.Featured[j].Priority
	})
	sort.Slice(result.Opinions, func(i, j int) bool {
		return result.Opinions[i].Priority < result.Opinions[j].Priority
	})

	return result
}

// truncateStories limits the total number of stories (headline + featured + opinions)
// to maxStories. Headline is always kept, featured and opinions are truncated proportionally.
func truncateStories(groups *GroupedStories, maxStories int) {
	if maxStories <= 0 {
		return // No limit
	}

	// Headline counts as 1 if present
	headlineCount := 0
	if groups.Headline != nil {
		headlineCount = 1
	}

	// Calculate how many featured + opinions we can keep
	remaining := maxStories - headlineCount
	if remaining <= 0 {
		// Only room for headline
		groups.Featured = nil
		groups.Opinions = nil
		return
	}

	totalNonHeadline := len(groups.Featured) + len(groups.Opinions)
	if totalNonHeadline <= remaining {
		return // Already within limit
	}

	// Truncate: prioritize featured over opinions, but keep at least 1 opinion if there are any
	if len(groups.Opinions) > 0 && remaining > 0 {
		// Keep at least 1 opinion
		featuredSlots := remaining - 1
		if len(groups.Featured) <= featuredSlots {
			// All featured fit, truncate opinions
			opinionSlots := remaining - len(groups.Featured)
			if len(groups.Opinions) > opinionSlots {
				groups.Opinions = groups.Opinions[:opinionSlots]
			}
		} else {
			// Truncate featured, keep 1 opinion
			groups.Featured = groups.Featured[:featuredSlots]
			groups.Opinions = groups.Opinions[:1]
		}
	} else {
		// No opinions, just truncate featured
		if len(groups.Featured) > remaining {
			groups.Featured = groups.Featured[:remaining]
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

// CompileDigestHTML generates an HTML newspaper-style digest with interleaved content
func CompileDigestHTML(
	posts []Post,
	cats Categories,
	storyGroups StoryGroups,
	newspaperConfig NewspaperConfig,
	contentPicks AllContentPicks,
	config Config,
) (string, error) {
	var html strings.Builder

	// Build post index for quick lookup
	postIndex := make(map[string]Post)
	for _, post := range posts {
		postIndex[post.Rkey] = post
	}

	// Collect content posts to interleave (same selection as markdown)
	var contentQueue []Post
	for _, section := range newspaperConfig.Sections {
		if section.Type != "content" {
			continue
		}
		sectionPosts := getSectionPosts(section.ID, cats, postIndex)
		if len(sectionPosts) == 0 {
			continue
		}

		// Add top 3 by likes
		popular := sortByLikes(sectionPosts)
		for i := 0; i < min(3, len(popular)); i++ {
			contentQueue = append(contentQueue, popular[i])
		}

		// Add top 3 by engagement (avoid duplicates)
		seen := make(map[string]bool)
		for _, p := range contentQueue {
			seen[p.Rkey] = true
		}
		engaging := sortByEngagement(sectionPosts)
		addedEngaging := 0
		for i := 0; i < len(engaging) && addedEngaging < 3; i++ {
			if !seen[engaging[i].Rkey] {
				contentQueue = append(contentQueue, engaging[i])
				seen[engaging[i].Rkey] = true
				addedEngaging++
			}
		}

		// Add sui generis picks (avoid duplicates)
		if picks, ok := contentPicks[section.ID]; ok {
			for _, rkey := range picks.SuiGeneris {
				if post, ok := postIndex[rkey]; ok && !seen[rkey] {
					contentQueue = append(contentQueue, post)
					seen[rkey] = true
				}
			}
		}
	}
	// Get news sections for interleaving calculation
	var newsSections []NewspaperSection
	for _, section := range newspaperConfig.Sections {
		if section.Type == "news" && section.ID != "front-page" {
			newsSections = append(newsSections, section)
		}
	}

	// Count grid gaps first (prioritize filling grids over interleaving)
	frontPageGroups := getFrontPageGroups(storyGroups)

	// Apply truncation based on max_stories from newspaper.json
	for _, section := range newspaperConfig.Sections {
		if section.ID == "front-page" && section.MaxStories > 0 {
			truncateStories(&frontPageGroups, section.MaxStories)
			break
		}
	}

	gridGapsNeeded := 0

	// Front page grid gap
	fpCount := len(frontPageGroups.Featured) + len(frontPageGroups.Opinions)
	if fpCount > 0 && fpCount%2 == 1 {
		gridGapsNeeded++
	}

	// News section grid gaps
	for _, section := range newsSections {
		groups := getSectionGroups(storyGroups, section.ID)
		// Apply truncation BEFORE counting (must match render phase)
		if section.MaxStories > 0 {
			truncateStories(&groups, section.MaxStories)
		}
		if groups.Headline == nil && len(groups.Featured) == 0 && len(groups.Opinions) == 0 {
			continue
		}
		count := len(groups.Featured) + len(groups.Opinions)
		if count > 0 && count%2 == 1 {
			gridGapsNeeded++
		}
	}

	// Split content: first N for grid gaps, rest for interleaving
	gridReserve := min(gridGapsNeeded, len(contentQueue))
	gridIdx := 0
	interleaveIdx := gridReserve

	// Calculate content insertion points for interleaving
	interleaveCount := len(contentQueue) - gridReserve
	contentPerGap := 1
	if len(newsSections) > 0 && interleaveCount > 0 {
		contentPerGap = (interleaveCount + len(newsSections)) / (len(newsSections) + 1)
		if contentPerGap < 1 {
			contentPerGap = 1
		}
	}

	// HTML head with inline CSS
	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>The Daily Digest - ` + escapeHTML(formatDigestDate(config.CreatedAt)) + `</title>
<style>
:root {
  --bg: #f5f1e8;
  --text: #1a1a1a;
  --muted: #555;
  --border: #d4cfc4;
  --accent: #8b0000;
  --section-bg: #ece7db;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: Georgia, "Times New Roman", serif;
  background: var(--bg);
  color: var(--text);
  line-height: 1.6;
  max-width: min(1200px, 90vw);
  margin: 0 auto;
  padding: clamp(1rem, 2vw, 3rem);
  font-size: clamp(1rem, 0.9rem + 0.5vw, 1.25rem);
}
h1 {
  font-size: clamp(2rem, 1.5rem + 2vw, 3rem);
  text-align: center;
  border-bottom: 3px double var(--text);
  padding-bottom: 0.5rem;
  margin-bottom: 0.25rem;
}
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.subtitle { text-align: center; color: var(--muted); margin-bottom: 2rem; }

/* Section headers - distinctive */
.section > summary { list-style: none; cursor: pointer; }
.section > summary::-webkit-details-marker { display: none; }
.section > summary h2 {
  text-align: center;
  font-size: clamp(1.2rem, 1rem + 1vw, 1.8rem);
  font-weight: 700;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  background: var(--section-bg);
  padding: 0.75rem 1rem;
  margin: 2rem 0 1.5rem;
  border-top: 3px double var(--border);
  border-bottom: 3px double var(--border);
}
.section > summary h2::after { content: " ▼"; font-size: 0.7em; color: var(--muted); }
.section:not([open]) > summary h2::after { content: " ▶"; }

/* Headline story - full width, prominent */
.headline-story { margin-bottom: 1.5rem; }
.headline-story h3 {
  font-size: clamp(1.3rem, 1rem + 1vw, 1.8rem);
  line-height: 1.3;
  margin-bottom: 0.5rem;
}
.headline-story h3 a { color: var(--text); }
.headline-story h3 a:hover { color: var(--accent); }

/* Stories grid - 2 columns */
.stories-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: clamp(1rem, 2vw, 2rem) clamp(1.5rem, 3vw, 3rem);
  margin-bottom: 1.5rem;
}
.story { border-top: 1px solid var(--border); padding-top: 0.75rem; }
.story h4 {
  font-size: clamp(1rem, 0.9rem + 0.5vw, 1.3rem);
  line-height: 1.3;
  margin-bottom: 0.25rem;
}
.story h4 a { color: var(--text); }
.story h4 a:hover { color: var(--accent); }

/* Expanded post list */
.story details, .headline-story details { margin-top: 0.25rem; }
.story details[open] summary, .headline-story details[open] summary { margin-bottom: 0.5rem; }
.story summary, .headline-story summary { font-size: 0.85em; color: var(--muted); cursor: pointer; list-style: none; }
.story summary::-webkit-details-marker, .headline-story summary::-webkit-details-marker { display: none; }
.story summary::before, .headline-story summary::before { content: "▶ "; font-size: 0.8em; }
.story details[open] summary::before, .headline-story details[open] summary::before { content: "▼ "; }
.post { margin: 0.5rem 0; padding-left: 0.75rem; border-left: 2px solid var(--border); }
.post .author { color: var(--accent); }
.post .text { color: var(--text); margin: 0.25rem 0; }
.post .meta { color: var(--muted); font-size: 0.85em; }

/* From the Feed section */
.content-break { margin: 2rem 0; padding: 1rem 0; border-top: 1px solid var(--border); border-bottom: 1px solid var(--border); }
.content-break-label { font-size: 0.75em; text-transform: uppercase; letter-spacing: 0.15em; color: var(--muted); margin-bottom: 1rem; }

/* Feed card - used everywhere for "From the Feed" posts */
.feed-card {
  background: var(--section-bg);
  padding: 1rem;
  border-radius: 4px;
  border: 1px solid var(--border);
  margin-bottom: 0.75rem;
}
.feed-card .label {
  font-size: 0.7em;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--muted);
  margin-bottom: 0.5rem;
}
.feed-card .author { font-weight: bold; }
.feed-card .text { margin: 0.25rem 0; }
.feed-card .meta { color: var(--muted); font-size: 0.85em; }
.feed-card .images img { max-width: 100%; max-height: 150px; margin: 0.5rem 0; cursor: pointer; }

/* Lightbox */
.lightbox { display: none; position: fixed; inset: 0; background: rgba(0,0,0,0.9); z-index: 1000; align-items: center; justify-content: center; }
.lightbox.open { display: flex; }
.lightbox img { max-width: 90vw; max-height: 90vh; object-fit: contain; }
.lightbox-close { position: absolute; top: 1rem; right: 1rem; color: white; font-size: 2rem; cursor: pointer; background: none; border: none; padding: 0.5rem; }
.lightbox-nav { position: absolute; top: 50%; transform: translateY(-50%); color: white; font-size: 3rem; cursor: pointer; background: none; border: none; padding: 1rem; opacity: 0.7; }
.lightbox-nav:hover { opacity: 1; }
.lightbox-nav.prev { left: 1rem; }
.lightbox-nav.next { right: 1rem; }
.lightbox-nav:disabled { opacity: 0.2; cursor: default; }
.lightbox-counter { position: absolute; bottom: 1rem; left: 50%; transform: translateX(-50%); color: white; font-size: 0.9rem; }
.images img { cursor: pointer; }

@media (max-width: 600px) {
  .stories-grid { grid-template-columns: 1fr; }
}
@media print { body { max-width: none; background: white; } }
</style>
</head>
<body>
`)

	// Title
	html.WriteString("<h1>The Daily Digest</h1>\n")
	html.WriteString(fmt.Sprintf("<p class=\"subtitle\">%s</p>\n", escapeHTML(formatDigestDate(config.CreatedAt))))

	// Front Page
	html.WriteString("<details class=\"section\" open>\n")
	html.WriteString("<summary><h2>Front Page</h2></summary>\n")

	// frontPageGroups already computed above for grid gap counting

	// Headline story (full width, prominent)
	if frontPageGroups.Headline != nil {
		writeHeadlineStory(&html, frontPageGroups.Headline, postIndex)
	}

	// Featured + Opinions in grid (same style)
	if len(frontPageGroups.Featured) > 0 || len(frontPageGroups.Opinions) > 0 {
		gridCount := len(frontPageGroups.Featured) + len(frontPageGroups.Opinions)
		html.WriteString("<div class=\"stories-grid\">\n")
		for _, group := range frontPageGroups.Featured {
			writeGridStory(&html, group, postIndex, false)
		}
		for _, group := range frontPageGroups.Opinions {
			writeGridStory(&html, group, postIndex, true)
		}
		// Fill grid gap with feed card if odd count (use reserved grid posts)
		if gridCount%2 == 1 && gridIdx < gridReserve {
			writeFeedCard(&html, contentQueue[gridIdx])
			gridIdx++
		}
		html.WriteString("</div>\n")
	}
	html.WriteString("</details>\n")

	// News sections with interleaved content
	renderedSections := 0
	for _, section := range newsSections {
		sectionGroups := getSectionGroups(storyGroups, section.ID)

		// Apply truncation based on max_stories
		if section.MaxStories > 0 {
			truncateStories(&sectionGroups, section.MaxStories)
		}

		// Skip empty sections
		if sectionGroups.Headline == nil && len(sectionGroups.Featured) == 0 && len(sectionGroups.Opinions) == 0 {
			continue
		}

		// Insert content BEFORE this section (as a break between sections)
		if renderedSections > 0 && interleaveIdx < len(contentQueue) {
			html.WriteString("<section class=\"content-break\">\n")
			html.WriteString("<div class=\"content-break-label\">From the Feed</div>\n")
			for i := 0; i < contentPerGap && interleaveIdx < len(contentQueue); i++ {
				writeFeedCard(&html, contentQueue[interleaveIdx])
				interleaveIdx++
			}
			html.WriteString("</section>\n")
		}

		html.WriteString("<details class=\"section\" open>\n")
		html.WriteString(fmt.Sprintf("<summary><h2>%s</h2></summary>\n", escapeHTML(section.Name)))

		// Headline story (full width, prominent)
		if sectionGroups.Headline != nil {
			writeHeadlineStory(&html, sectionGroups.Headline, postIndex)
		}

		// Featured + Opinions in grid (same style)
		if len(sectionGroups.Featured) > 0 || len(sectionGroups.Opinions) > 0 {
			gridCount := len(sectionGroups.Featured) + len(sectionGroups.Opinions)
			html.WriteString("<div class=\"stories-grid\">\n")
			for _, group := range sectionGroups.Featured {
				writeGridStory(&html, group, postIndex, false)
			}
			for _, group := range sectionGroups.Opinions {
				writeGridStory(&html, group, postIndex, true)
			}
			// Fill grid gap with feed card if odd count (use reserved grid posts)
			if gridCount%2 == 1 && gridIdx < gridReserve {
				writeFeedCard(&html, contentQueue[gridIdx])
				gridIdx++
			}
			html.WriteString("</div>\n")
		}

		html.WriteString("</details>\n")
		renderedSections++
	}

	// Any remaining content at the end
	if interleaveIdx < len(contentQueue) {
		html.WriteString("<section class=\"content-break\">\n")
		html.WriteString("<div class=\"content-break-label\">From the Feed</div>\n")
		for interleaveIdx < len(contentQueue) {
			writeFeedCard(&html, contentQueue[interleaveIdx])
			interleaveIdx++
		}
		html.WriteString("</section>\n")
	}

	// Lightbox HTML and JavaScript
	html.WriteString(`<div id="lightbox" class="lightbox" onclick="closeLightbox(event)">
<button class="lightbox-close" onclick="closeLightbox(event)">&times;</button>
<button class="lightbox-nav prev" onclick="navLightbox(-1, event)">&#8249;</button>
<img id="lightbox-img" src="" alt="">
<button class="lightbox-nav next" onclick="navLightbox(1, event)">&#8250;</button>
<div class="lightbox-counter"><span id="lightbox-idx">1</span> / <span id="lightbox-total">1</span></div>
</div>
<script>
let lbGroup = [], lbIdx = 0;
function openLightbox(img) {
  const group = img.dataset.group;
  lbGroup = Array.from(document.querySelectorAll('img[data-group="' + group + '"]'));
  lbIdx = parseInt(img.dataset.idx) || 0;
  showLightboxImage();
  document.getElementById('lightbox').classList.add('open');
  document.body.style.overflow = 'hidden';
}
function closeLightbox(e) {
  if (e.target.id === 'lightbox' || e.target.classList.contains('lightbox-close')) {
    document.getElementById('lightbox').classList.remove('open');
    document.body.style.overflow = '';
  }
}
function navLightbox(dir, e) {
  e.stopPropagation();
  lbIdx = Math.max(0, Math.min(lbGroup.length - 1, lbIdx + dir));
  showLightboxImage();
}
function showLightboxImage() {
  const img = lbGroup[lbIdx];
  document.getElementById('lightbox-img').src = img.src;
  document.getElementById('lightbox-img').alt = img.alt;
  document.getElementById('lightbox-idx').textContent = lbIdx + 1;
  document.getElementById('lightbox-total').textContent = lbGroup.length;
  document.querySelector('.lightbox-nav.prev').disabled = lbIdx === 0;
  document.querySelector('.lightbox-nav.next').disabled = lbIdx === lbGroup.length - 1;
}
document.addEventListener('keydown', function(e) {
  if (!document.getElementById('lightbox').classList.contains('open')) return;
  if (e.key === 'Escape') { document.getElementById('lightbox').classList.remove('open'); document.body.style.overflow = ''; }
  if (e.key === 'ArrowLeft') navLightbox(-1, e);
  if (e.key === 'ArrowRight') navLightbox(1, e);
});
</script>
`)
	html.WriteString("</body>\n</html>\n")

	return html.String(), nil
}

// writeHeadlineStory writes a headline story (full width, prominent)
func writeHeadlineStory(html *strings.Builder, group *StoryGroup, postIndex map[string]Post) {
	post := postIndex[group.PrimaryRkey]
	html.WriteString("<article class=\"headline-story\">\n")
	html.WriteString(fmt.Sprintf("<h3><a href=\"%s\">%s</a></h3>\n", escapeHTML(postURL(post)), escapeHTML(group.Headline)))
	if group.Summary != "" {
		html.WriteString(fmt.Sprintf("<p>%s</p>\n", escapeHTML(group.Summary)))
	}

	// Always show posts expanded
	html.WriteString(fmt.Sprintf("<details open><summary>%d posts in story</summary>\n", len(group.PostRkeys)))
	for _, rkey := range group.PostRkeys {
		if p, ok := postIndex[rkey]; ok {
			writeStoryPost(html, p)
		}
	}
	html.WriteString("</details>\n")
	html.WriteString("</article>\n")
}

// writeGridStory writes a featured or opinion story in the grid
func writeGridStory(html *strings.Builder, group *StoryGroup, postIndex map[string]Post, isOpinion bool) {
	post := postIndex[group.PrimaryRkey]
	html.WriteString("<article class=\"story\">\n")

	headline := group.Headline
	if isOpinion {
		headline = "Opinion: " + headline
	}
	html.WriteString(fmt.Sprintf("<h4><a href=\"%s\">%s</a></h4>\n", escapeHTML(postURL(post)), escapeHTML(headline)))

	// Always show posts expanded
	html.WriteString(fmt.Sprintf("<details open><summary>%d posts</summary>\n", len(group.PostRkeys)))
	for _, rkey := range group.PostRkeys {
		if p, ok := postIndex[rkey]; ok {
			writeStoryPost(html, p)
		}
	}
	html.WriteString("</details>\n")
	html.WriteString("</article>\n")
}

// writeStoryPost writes a post within a story's details section
func writeStoryPost(html *strings.Builder, post Post) {
	html.WriteString("<div class=\"post\">\n")
	html.WriteString(fmt.Sprintf("<span class=\"author\">@%s</span>\n", escapeHTML(post.Author.Handle)))
	html.WriteString(fmt.Sprintf("<p class=\"text\">%s</p>\n", escapeHTML(post.Text)))

	// Images
	if len(post.Images) > 0 {
		html.WriteString("<div class=\"images\">\n")
		for i, img := range post.Images {
			alt := img.Alt
			if alt == "" {
				alt = "Image"
			}
			html.WriteString(fmt.Sprintf("<img src=\"%s\" alt=\"%s\" loading=\"lazy\" style=\"max-width:100%%;max-height:200px;\" data-group=\"%s\" data-idx=\"%d\" onclick=\"openLightbox(this)\">\n",
				escapeHTML(img.URL), escapeHTML(alt), escapeHTML(post.Rkey), i))
		}
		html.WriteString("</div>\n")
	}

	html.WriteString(fmt.Sprintf("<p class=\"meta\">%s • ♥ %d • <a href=\"%s\">View</a></p>\n",
		escapeHTML(formatPostTime(post.CreatedAt)), post.LikeCount, escapeHTML(postURL(post))))
	html.WriteString("</div>\n")
}

// writeFeedCard writes a "From the Feed" post with box styling (used everywhere)
func writeFeedCard(html *strings.Builder, post Post) {
	html.WriteString("<article class=\"feed-card\">\n")
	html.WriteString("<div class=\"label\">From the Feed</div>\n")
	html.WriteString(fmt.Sprintf("<a class=\"author\" href=\"%s\">@%s</a>\n", escapeHTML(postURL(post)), escapeHTML(post.Author.Handle)))
	html.WriteString(fmt.Sprintf("<p class=\"text\">%s</p>\n", escapeHTML(post.Text)))

	// Images
	if len(post.Images) > 0 {
		html.WriteString("<div class=\"images\">\n")
		for i, img := range post.Images {
			alt := img.Alt
			if alt == "" {
				alt = "Image"
			}
			html.WriteString(fmt.Sprintf("<img src=\"%s\" alt=\"%s\" loading=\"lazy\" data-group=\"%s\" data-idx=\"%d\" onclick=\"openLightbox(this)\">\n",
				escapeHTML(img.URL), escapeHTML(alt), escapeHTML(post.Rkey), i))
		}
		html.WriteString("</div>\n")
	}

	html.WriteString(fmt.Sprintf("<p class=\"meta\">%s • ♥ %d • <a href=\"%s\">View</a></p>\n",
		escapeHTML(formatPostTime(post.CreatedAt)), post.LikeCount, escapeHTML(postURL(post))))
	html.WriteString("</article>\n")
}
