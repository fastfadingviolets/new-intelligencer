package main

import (
	"fmt"
	"strings"
)

// BuildDigestData converts raw input types into a template-ready DigestData struct.
// All HTML escaping and formatting is done here; templates just output values directly.
func BuildDigestData(
	posts []Post,
	cats Categories,
	storyGroups StoryGroups,
	newspaperConfig NewspaperConfig,
	contentPicks AllContentPicks,
	config Config,
) *DigestData {
	// Build post index for quick lookup
	postIndex := make(map[string]Post)
	for _, post := range posts {
		postIndex[post.Rkey] = post
	}

	// Build thread graph (parentRkey → []childRkeys)
	threadReplies := make(map[string][]string)
	for _, post := range posts {
		if post.ReplyTo != nil && post.ReplyTo.URI != "" {
			parentRkey := extractRkeyFromURI(post.ReplyTo.URI)
			if parentRkey != "" {
				threadReplies[parentRkey] = append(threadReplies[parentRkey], post.Rkey)
			}
		}
	}

	// Collect content posts to interleave
	contentQueue := collectContentPosts(newspaperConfig, cats, postIndex, contentPicks)

	// Get news sections (excluding front-page)
	var newsSections []NewspaperSection
	for _, section := range newspaperConfig.Sections {
		if section.Type == "news" && section.ID != "front-page" {
			newsSections = append(newsSections, section)
		}
	}

	// Pre-compute grid gaps and split content for filling vs interleaving
	frontPageGroups := getFrontPageGroups(storyGroups)
	for _, section := range newspaperConfig.Sections {
		if section.ID == "front-page" && section.MaxStories > 0 {
			truncateStories(&frontPageGroups, section.MaxStories)
			break
		}
	}

	gridGapsNeeded := countGridGaps(frontPageGroups, newsSections, storyGroups)
	gridReserve := min(gridGapsNeeded, len(contentQueue))

	// Split content: first N for grid gaps, rest for interleaving
	gridIdx := 0
	interleaveIdx := gridReserve

	// Calculate content per interleave gap
	interleaveCount := len(contentQueue) - gridReserve
	contentPerGap := 1
	if len(newsSections) > 0 && interleaveCount > 0 {
		contentPerGap = (interleaveCount + len(newsSections)) / (len(newsSections) + 1)
		if contentPerGap < 1 {
			contentPerGap = 1
		}
	}

	// Build sidebar sections
	sidebarSections := buildSidebarSections(newsSections, storyGroups)

	// Build front page section
	frontPage := buildSection("front-page", "Front Page", frontPageGroups, postIndex, threadReplies, contentQueue, &gridIdx, gridReserve)

	// Build news sections with interleaved content
	var renderedSections []*RenderedSection
	renderedCount := 0
	for _, section := range newsSections {
		sectionGroups := getSectionGroups(storyGroups, section.ID)
		if section.MaxStories > 0 {
			truncateStories(&sectionGroups, section.MaxStories)
		}

		// Skip empty sections
		if sectionGroups.Headline == nil && len(sectionGroups.Stories) == 0 && len(sectionGroups.Opinions) == 0 {
			continue
		}

		// Collect content to interleave BEFORE this section
		var contentBefore []*RenderedFeedCard
		if renderedCount > 0 && interleaveIdx < len(contentQueue) {
			for i := 0; i < contentPerGap && interleaveIdx < len(contentQueue); i++ {
				contentBefore = append(contentBefore, buildFeedCard(contentQueue[interleaveIdx], postIndex, threadReplies))
				interleaveIdx++
			}
		}

		// Build the section
		rendered := buildSection(section.ID, section.Name, sectionGroups, postIndex, threadReplies, contentQueue, &gridIdx, gridReserve)
		rendered.ContentBefore = contentBefore
		renderedSections = append(renderedSections, rendered)
		renderedCount++
	}

	// Any remaining content at the end
	var trailingContent []*RenderedFeedCard
	for interleaveIdx < len(contentQueue) {
		trailingContent = append(trailingContent, buildFeedCard(contentQueue[interleaveIdx], postIndex, threadReplies))
		interleaveIdx++
	}

	return &DigestData{
		Title:           "The New Intelligencer",
		Date:            formatDigestDate(config.CreatedAt),
		Sections:        sidebarSections,
		FrontPage:       frontPage,
		NewsSections:    renderedSections,
		TrailingContent: trailingContent,
	}
}

// collectContentPosts gathers content posts for interleaving (same logic as CompileDigestHTML).
func collectContentPosts(
	newspaperConfig NewspaperConfig,
	cats Categories,
	postIndex map[string]Post,
	contentPicks AllContentPicks,
) []Post {
	var contentQueue []Post

	for _, section := range newspaperConfig.Sections {
		if section.Type != "content" {
			continue
		}
		sectionPosts := getSectionPosts(section.ID, cats, postIndex)
		if len(sectionPosts) == 0 {
			continue
		}

		// Add top N by likes
		popular := sortByLikes(sectionPosts)
		for i := 0; i < min(section.MaxStories, len(popular)); i++ {
			contentQueue = append(contentQueue, popular[i])
		}

		// Add top N by engagement (avoid duplicates)
		seen := make(map[string]bool)
		for _, p := range contentQueue {
			seen[p.Rkey] = true
		}
		engaging := sortByEngagement(sectionPosts)
		addedEngaging := 0
		for i := 0; i < len(engaging) && addedEngaging < section.MaxStories; i++ {
			if !seen[engaging[i].Rkey] {
				contentQueue = append(contentQueue, engaging[i])
				seen[engaging[i].Rkey] = true
				addedEngaging++
			}
		}

		// Add sui generis picks (avoid duplicates)
		if picks, ok := contentPicks[section.ID]; ok {
			addedSuiGeneris := 0
			for _, rkey := range picks.SuiGeneris {
				if addedSuiGeneris >= section.MaxStories {
					break
				}
				if post, ok := postIndex[rkey]; ok && !seen[rkey] {
					contentQueue = append(contentQueue, post)
					seen[rkey] = true
					addedSuiGeneris++
				}
			}
		}
	}

	return contentQueue
}

// countGridGaps counts how many odd-count grids need a filler card.
func countGridGaps(frontPageGroups GroupedStories, newsSections []NewspaperSection, storyGroups StoryGroups) int {
	count := 0

	// Front page grid gap
	fpCount := len(frontPageGroups.Stories) + len(frontPageGroups.Opinions)
	if fpCount > 0 && fpCount%2 == 1 {
		count++
	}

	// News section grid gaps
	for _, section := range newsSections {
		groups := getSectionGroups(storyGroups, section.ID)
		if section.MaxStories > 0 {
			truncateStories(&groups, section.MaxStories)
		}
		if groups.Headline == nil && len(groups.Stories) == 0 && len(groups.Opinions) == 0 {
			continue
		}
		gridCount := len(groups.Stories) + len(groups.Opinions)
		if gridCount > 0 && gridCount%2 == 1 {
			count++
		}
	}

	return count
}

// buildSidebarSections creates the navigation section list.
func buildSidebarSections(newsSections []NewspaperSection, storyGroups StoryGroups) []SectionNav {
	sections := []SectionNav{
		{ID: "front-page", Name: "Front Page", Slug: slugify("front-page")},
	}

	for _, section := range newsSections {
		groups := getSectionGroups(storyGroups, section.ID)
		if section.MaxStories > 0 {
			truncateStories(&groups, section.MaxStories)
		}
		if groups.Headline != nil || len(groups.Stories) > 0 || len(groups.Opinions) > 0 {
			sections = append(sections, SectionNav{
				ID:   section.ID,
				Name: section.Name,
				Slug: slugify(section.ID),
			})
		}
	}

	return sections
}

// buildSection builds a single rendered section.
func buildSection(
	id string,
	name string,
	groups GroupedStories,
	postIndex map[string]Post,
	threadReplies map[string][]string,
	contentQueue []Post,
	gridIdx *int,
	gridReserve int,
) *RenderedSection {
	section := &RenderedSection{
		ID:   id,
		Name: name,
		Slug: slugify(id),
	}

	// Headline story
	if groups.Headline != nil {
		section.HeadlineStory = buildStory(groups.Headline, postIndex, threadReplies, false)
	}

	// Grid stories (stories + opinions)
	for _, group := range groups.Stories {
		section.GridStories = append(section.GridStories, buildStory(group, postIndex, threadReplies, false))
	}
	for _, group := range groups.Opinions {
		section.GridStories = append(section.GridStories, buildStory(group, postIndex, threadReplies, true))
	}

	// Fill grid gap with feed card if odd count
	gridCount := len(groups.Stories) + len(groups.Opinions)
	if gridCount > 0 && gridCount%2 == 1 && *gridIdx < gridReserve {
		filler := buildFeedCard(contentQueue[*gridIdx], postIndex, threadReplies)
		section.GridStories = append(section.GridStories, &RenderedStory{
			IsFiller: true,
			Filler:   filler,
		})
		*gridIdx++
	}

	return section
}

// buildStory builds a rendered story from a story group.
func buildStory(group *StoryGroup, postIndex map[string]Post, threadReplies map[string][]string, isOpinion bool) *RenderedStory {
	primaryPost := postIndex[group.PrimaryRkey]

	story := &RenderedStory{
		Headline:  escapeHTML(getHeadline(group, primaryPost)),
		URL:       postURL(primaryPost),
		Summary:   escapeHTML(group.Summary),
		IsOpinion: isOpinion || group.IsOpinion,
		PostCount: len(group.PostRkeys),
	}

	// Build posts
	for _, rkey := range group.PostRkeys {
		if post, ok := postIndex[rkey]; ok {
			story.Posts = append(story.Posts, buildPost(post, postIndex, threadReplies))
		}
	}

	return story
}

// buildPost builds a rendered post.
func buildPost(post Post, postIndex map[string]Post, threadReplies map[string][]string) *RenderedPost {
	rendered := &RenderedPost{
		AuthorHandle: escapeHTML(post.Author.Handle),
		AuthorName:   escapeHTML(post.Author.DisplayName),
		AuthorURL:    fmt.Sprintf("https://bsky.app/profile/%s", post.Author.Handle),
		Text:         formatPostText(post.Text),
		Time:         formatPostTime(post.CreatedAt),
		URL:          postURL(post),
		LikeCount:    int(post.LikeCount),
		RepostCount:  int(post.RepostCount),
	}

	// Repost info
	if post.Repost != nil {
		rendered.IsRepost = true
		rendered.RepostByHandle = escapeHTML(post.Repost.ByHandle)
	}

	// Images
	for i, img := range post.Images {
		alt := img.Alt
		if alt == "" {
			alt = "Image"
		}
		rendered.Images = append(rendered.Images, RenderedImage{
			URL:     img.URL,
			Alt:     escapeHTML(alt),
			Thumb:   img.URL,
			GroupID: post.Rkey,
			Index:   i,
		})
	}

	// External link
	if post.ExternalLink != nil {
		desc := ""
		if post.ExternalLink.Description != "" {
			desc = truncateText(post.ExternalLink.Description, 120)
		}
		rendered.ExternalLink = &RenderedLink{
			URL:         post.ExternalLink.URL,
			Title:       escapeHTML(post.ExternalLink.Title),
			Description: escapeHTML(desc),
			Thumb:       post.ExternalLink.Thumb,
			Domain:      extractDomain(post.ExternalLink.URL),
		}
	}

	// Quote
	if post.Quote != nil {
		quoteURL := fmt.Sprintf("https://bsky.app/profile/%s/post/%s", post.Quote.Author.Handle, post.Quote.Rkey)
		rendered.Quote = &RenderedQuote{
			AuthorHandle: escapeHTML(post.Quote.Author.Handle),
			AuthorName:   escapeHTML(post.Quote.Author.DisplayName),
			AuthorURL:    fmt.Sprintf("https://bsky.app/profile/%s", post.Quote.Author.Handle),
			Text:         escapeHTML(truncateText(post.Quote.Text, 120)),
			URL:          quoteURL,
		}
	}

	// Thread replies
	if replies := threadReplies[post.Rkey]; len(replies) > 0 {
		for _, replyRkey := range replies {
			if replyPost, ok := postIndex[replyRkey]; ok {
				// Recursively build reply (without its own thread replies to avoid infinite loops)
				rendered.ThreadReplies = append(rendered.ThreadReplies, buildPost(replyPost, postIndex, nil))
			}
		}
	}

	return rendered
}

// buildFeedCard builds a feed card for content interleaving.
func buildFeedCard(post Post, postIndex map[string]Post, threadReplies map[string][]string) *RenderedFeedCard {
	return &RenderedFeedCard{
		Post: buildPost(post, postIndex, threadReplies),
	}
}

// formatPostText escapes HTML and converts newlines to <br> tags.
func formatPostText(text string) string {
	escaped := escapeHTML(text)
	return strings.ReplaceAll(escaped, "\n", "<br>")
}
