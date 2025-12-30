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

	// Get sorted category names (only visible categories)
	catNames := make([]string, 0, len(cats))
	hiddenCatNames := make([]string, 0)
	for name, catData := range cats {
		if catData.IsHidden {
			hiddenCatNames = append(hiddenCatNames, name)
		} else if len(catData.Visible) > 0 { // Only include non-hidden categories with visible posts
			catNames = append(catNames, name)
		}
	}
	sort.Strings(catNames)
	sort.Strings(hiddenCatNames)

	// Build index for quick post lookup
	postIndex := make(map[string]Post)
	for _, post := range posts {
		postIndex[post.Rkey] = post
	}

	// Track which posts are referenced (for references section)
	referenced := make(map[string]bool)

	// Category sections
	for _, catName := range catNames {
		catData := cats[catName]
		postCount := len(catData.Visible)
		pluralS := "s"
		if postCount == 1 {
			pluralS = ""
		}

		md.WriteString(fmt.Sprintf("## %s (%d post%s)\n\n", catName, postCount, pluralS))

		// Add summary if available
		if summary, ok := sums[catName]; ok && summary != "" {
			md.WriteString(summary)
			md.WriteString("\n\n")

			// Track referenced posts (only visible posts can be referenced)
			for _, rkey := range catData.Visible {
				if strings.Contains(summary, "["+rkey+"]") {
					referenced[rkey] = true
				}
			}
		} else {
			// No summary available
			md.WriteString("(No summary available)\n\n")
		}

		// Add note about hidden posts if any
		if len(catData.Hidden) > 0 {
			hiddenCount := len(catData.Hidden)
			reason := catData.HiddenReason
			if reason != "" {
				md.WriteString(fmt.Sprintf("*Note: %d posts filtered (%s)*\n\n", hiddenCount, reason))
			} else {
				md.WriteString(fmt.Sprintf("*Note: %d posts filtered*\n\n", hiddenCount))
			}
		}
	}

	// Hidden categories section (if any)
	if len(hiddenCatNames) > 0 {
		md.WriteString("## Hidden Categories\n\n")
		for _, catName := range hiddenCatNames {
			catData := cats[catName]
			postCount := len(catData.Visible) + len(catData.Hidden)
			md.WriteString(fmt.Sprintf("- %s (%d posts)\n", catName, postCount))
		}
		md.WriteString("\n")
	}

	// Separator before references
	md.WriteString("---\n\n")

	// References section
	md.WriteString("## References\n\n")

	// Get all referenced posts in order they appear in categories
	var refPosts []Post
	seenRefs := make(map[string]bool)

	for _, catName := range catNames {
		for _, rkey := range cats[catName].Visible {
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

// CompileNewspaper generates a newspaper-style digest with front page and sections
func CompileNewspaper(
	posts []Post,
	cats Categories,
	sums Summaries,
	storyGroups StoryGroups,
	newspaperConfig NewspaperConfig,
	sectionAssignments SectionAssignments,
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
		md.WriteString(fmt.Sprintf("### TOP STORY: %s\n\n", headline.Headline))
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
			md.WriteString(fmt.Sprintf("- %s [%s]\n", group.Headline, group.PrimaryRkey))
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
			md.WriteString(fmt.Sprintf("> %s - @%s [%s]\n\n", snippet, post.Author.Handle, group.PrimaryRkey))
			for _, rkey := range group.PostRkeys {
				referenced[rkey] = true
			}
		}
	}

	md.WriteString("---\n\n")

	// Sections
	for _, section := range newspaperConfig.Sections {
		if section.Type == "news" {
			md.WriteString(fmt.Sprintf("## %s (News)\n\n", section.Name))

			// Get story groups for this section
			sectionGroups := getSectionGroups(storyGroups, section.ID)

			// Headline
			if sectionGroups.Headline != nil {
				headline := sectionGroups.Headline
				md.WriteString(fmt.Sprintf("### Headline: %s\n\n", headline.Headline))
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
					md.WriteString(fmt.Sprintf("%d. %s [%s]\n", i+1, group.Headline, group.PrimaryRkey))
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
					md.WriteString(fmt.Sprintf("- @%s: \"%s\" [%s]\n", post.Author.Handle, snippet, group.PrimaryRkey))
					for _, rkey := range group.PostRkeys {
						referenced[rkey] = true
					}
				}
				md.WriteString("\n")
			}

		} else if section.Type == "content" {
			md.WriteString(fmt.Sprintf("## %s (Content)\n\n", section.Name))

			// Get all posts in this section's categories
			sectionPosts := getSectionPosts(section.ID, sectionAssignments, cats, postIndex)

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
		if !group.IsFrontPage {
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

	for id := range groups {
		group := groups[id]
		if group.SectionID != sectionID {
			continue
		}
		if group.IsFrontPage {
			continue // Skip front page exclusives
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

func getSectionPosts(sectionID string, assignments SectionAssignments, cats Categories, postIndex map[string]Post) []Post {
	var posts []Post
	seen := make(map[string]bool)

	categoryNames := assignments[sectionID]
	for _, catName := range categoryNames {
		if catData, ok := cats[catName]; ok {
			for _, rkey := range catData.Visible {
				if !seen[rkey] {
					if post, ok := postIndex[rkey]; ok {
						posts = append(posts, post)
						seen[rkey] = true
					}
				}
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
