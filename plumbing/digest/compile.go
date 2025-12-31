package main

import (
	"fmt"
	"sort"
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

	// Sort IDs for deterministic iteration
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
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

	return result
}

func getSectionGroups(groups StoryGroups, sectionID string) GroupedStories {
	result := GroupedStories{}

	// Sort IDs for deterministic iteration
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
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

	return result
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

// escapeHTML escapes special HTML characters
func escapeHTML(text string) string {
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
	contentIdx := 0

	// Get news sections for interleaving calculation
	var newsSections []NewspaperSection
	for _, section := range newspaperConfig.Sections {
		if section.Type == "news" && section.ID != "front-page" {
			newsSections = append(newsSections, section)
		}
	}

	// Calculate content insertion points
	contentPerGap := 1
	if len(newsSections) > 0 && len(contentQueue) > 0 {
		contentPerGap = (len(contentQueue) + len(newsSections)) / (len(newsSections) + 1)
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
  --bg: #faf9f6;
  --text: #1a1a1a;
  --muted: #666;
  --border: #ccc;
  --accent: #0066cc;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #1a1a1a;
    --text: #e8e6e3;
    --muted: #999;
    --border: #444;
    --accent: #4da6ff;
  }
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: Georgia, "Times New Roman", serif;
  background: var(--bg);
  color: var(--text);
  line-height: 1.5;
  max-width: 900px;
  margin: 0 auto;
  padding: 1rem 2rem;
}
h1 { font-size: 2.5rem; text-align: center; border-bottom: 2px solid var(--text); padding-bottom: 0.5rem; margin-bottom: 0.25rem; }
h2 { font-size: 1.3rem; text-transform: uppercase; letter-spacing: 0.05em; margin: 2rem 0 1rem; padding-bottom: 0.5rem; border-bottom: 1px solid var(--border); }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.subtitle { text-align: center; color: var(--muted); font-size: 0.9rem; margin-bottom: 2rem; }

/* Headline story - full width, prominent */
.headline-story { margin-bottom: 1.5rem; }
.headline-story h3 { font-size: 1.4rem; line-height: 1.3; margin-bottom: 0.5rem; }
.headline-story h3 a { color: var(--text); }
.headline-story h3 a:hover { color: var(--accent); }

/* Stories grid - 2 columns */
.stories-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem 2rem;
  margin-bottom: 1.5rem;
}
.story { border-top: 1px solid var(--border); padding-top: 0.75rem; }
.story h4 { font-size: 1rem; line-height: 1.3; margin-bottom: 0.25rem; }
.story h4 a { color: var(--text); }
.story h4 a:hover { color: var(--accent); }

/* Expanded post list */
details { margin-top: 0.25rem; }
details[open] summary { margin-bottom: 0.5rem; }
summary { font-size: 0.8rem; color: var(--muted); cursor: pointer; list-style: none; }
summary::-webkit-details-marker { display: none; }
summary::before { content: "▶ "; font-size: 0.7rem; }
details[open] summary::before { content: "▼ "; }
.post { margin: 0.5rem 0; padding-left: 0.75rem; border-left: 2px solid var(--border); font-size: 0.85rem; }
.post .author { color: var(--accent); }
.post .text { color: var(--text); margin: 0.25rem 0; }
.post .meta { color: var(--muted); font-size: 0.75rem; }

/* From the Feed section */
.content-break { margin: 2rem 0; padding: 1rem 0; border-top: 1px solid var(--border); border-bottom: 1px solid var(--border); }
.content-break-label { font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.15em; color: var(--muted); margin-bottom: 1rem; }
.feed-post { margin: 0.75rem 0; font-size: 0.9rem; }
.feed-post .author { font-weight: bold; }
.feed-post .text { margin: 0.25rem 0; }
.feed-post .meta { color: var(--muted); font-size: 0.8rem; }
.feed-post .images img { max-width: 100%; max-height: 150px; margin: 0.5rem 0; }

@media (max-width: 600px) {
  .stories-grid { grid-template-columns: 1fr; }
  body { padding: 1rem; }
}
@media print { body { max-width: none; } }
</style>
</head>
<body>
`)

	// Title
	html.WriteString("<h1>The Daily Digest</h1>\n")
	html.WriteString(fmt.Sprintf("<p class=\"subtitle\">%s</p>\n", escapeHTML(formatDigestDate(config.CreatedAt))))

	// Front Page
	html.WriteString("<h2>Front Page</h2>\n")

	frontPageGroups := getFrontPageGroups(storyGroups)

	// Headline story (full width, prominent)
	if frontPageGroups.Headline != nil {
		writeHeadlineStory(&html, frontPageGroups.Headline, postIndex)
	}

	// Featured + Opinions in grid (same style)
	if len(frontPageGroups.Featured) > 0 || len(frontPageGroups.Opinions) > 0 {
		html.WriteString("<div class=\"stories-grid\">\n")
		for _, group := range frontPageGroups.Featured {
			writeGridStory(&html, group, postIndex, false)
		}
		for _, group := range frontPageGroups.Opinions {
			writeGridStory(&html, group, postIndex, true)
		}
		html.WriteString("</div>\n")
	}

	// News sections with interleaved content
	renderedSections := 0
	for _, section := range newsSections {
		sectionGroups := getSectionGroups(storyGroups, section.ID)

		// Skip empty sections
		if sectionGroups.Headline == nil && len(sectionGroups.Featured) == 0 && len(sectionGroups.Opinions) == 0 {
			continue
		}

		// Insert content BEFORE this section (as a break between sections)
		if renderedSections > 0 && contentIdx < len(contentQueue) {
			html.WriteString("<section class=\"content-break\">\n")
			html.WriteString("<div class=\"content-break-label\">From the Feed</div>\n")
			for i := 0; i < contentPerGap && contentIdx < len(contentQueue); i++ {
				writeFeedPost(&html, contentQueue[contentIdx])
				contentIdx++
			}
			html.WriteString("</section>\n")
		}

		html.WriteString(fmt.Sprintf("<h2>%s</h2>\n", escapeHTML(section.Name)))

		// Headline story (full width, prominent)
		if sectionGroups.Headline != nil {
			writeHeadlineStory(&html, sectionGroups.Headline, postIndex)
		}

		// Featured + Opinions in grid (same style)
		if len(sectionGroups.Featured) > 0 || len(sectionGroups.Opinions) > 0 {
			html.WriteString("<div class=\"stories-grid\">\n")
			for _, group := range sectionGroups.Featured {
				writeGridStory(&html, group, postIndex, false)
			}
			for _, group := range sectionGroups.Opinions {
				writeGridStory(&html, group, postIndex, true)
			}
			html.WriteString("</div>\n")
		}

		renderedSections++
	}

	// Any remaining content at the end
	if contentIdx < len(contentQueue) {
		html.WriteString("<section class=\"content-break\">\n")
		html.WriteString("<div class=\"content-break-label\">From the Feed</div>\n")
		for contentIdx < len(contentQueue) {
			writeFeedPost(&html, contentQueue[contentIdx])
			contentIdx++
		}
		html.WriteString("</section>\n")
	}

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
		for _, img := range post.Images {
			alt := img.Alt
			if alt == "" {
				alt = "Image"
			}
			html.WriteString(fmt.Sprintf("<img src=\"%s\" alt=\"%s\" loading=\"lazy\" style=\"max-width:100%%;max-height:200px;\">\n", escapeHTML(img.URL), escapeHTML(alt)))
		}
		html.WriteString("</div>\n")
	}

	html.WriteString(fmt.Sprintf("<p class=\"meta\">%s • ♥ %d • <a href=\"%s\">View</a></p>\n",
		escapeHTML(formatPostTime(post.CreatedAt)), post.LikeCount, escapeHTML(postURL(post))))
	html.WriteString("</div>\n")
}

// writeFeedPost writes a "From the Feed" post (simpler style)
func writeFeedPost(html *strings.Builder, post Post) {
	html.WriteString("<div class=\"feed-post\">\n")
	html.WriteString(fmt.Sprintf("<span class=\"author\">@%s</span>\n", escapeHTML(post.Author.Handle)))
	html.WriteString(fmt.Sprintf("<p class=\"text\">%s</p>\n", escapeHTML(post.Text)))

	// Images
	if len(post.Images) > 0 {
		html.WriteString("<div class=\"images\">\n")
		for _, img := range post.Images {
			alt := img.Alt
			if alt == "" {
				alt = "Image"
			}
			html.WriteString(fmt.Sprintf("<img src=\"%s\" alt=\"%s\" loading=\"lazy\">\n", escapeHTML(img.URL), escapeHTML(alt)))
		}
		html.WriteString("</div>\n")
	}

	html.WriteString(fmt.Sprintf("<p class=\"meta\">%s • ♥ %d replies %d • <a href=\"%s\">View</a></p>\n",
		escapeHTML(formatPostTime(post.CreatedAt)), post.LikeCount, post.ReplyCount, escapeHTML(postURL(post))))
	html.WriteString("</div>\n")
}
