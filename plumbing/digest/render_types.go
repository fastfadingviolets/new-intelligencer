package main

// render_types.go contains data structures used for template rendering.
// These are pre-computed, template-ready versions of the raw data types.
// All HTML escaping and formatting is done during construction, so templates
// can output values directly without additional processing.

// DigestData contains everything needed to render the digest HTML.
type DigestData struct {
	// Metadata
	Title string // "The Daily Digest"
	Date  string // Formatted date string

	// Navigation - list of sections for sidebar
	Sections []SectionNav

	// Content sections
	FrontPage    *RenderedSection
	NewsSections []*RenderedSection

	// Remaining content not placed in sections
	TrailingContent []*RenderedFeedCard
}

// SectionNav represents a section link in the sidebar navigation.
type SectionNav struct {
	ID   string // Original section ID
	Name string // Display name (escaped)
	Slug string // URL-safe slug for anchor
}

// RenderedSection represents a fully-rendered section ready for templating.
type RenderedSection struct {
	ID            string
	Name          string // Display name (escaped)
	Slug          string // URL-safe slug for anchor
	HeadlineStory *RenderedStory
	GridStories   []*RenderedStory    // Stories + opinions for 2-column grid
	ContentBefore []*RenderedFeedCard // "From the Feed" content before this section
}

// RenderedStory represents a story ready for templating.
type RenderedStory struct {
	Headline  string // Escaped headline text
	URL       string // Link to primary post
	Summary   string // Escaped summary (may be empty)
	IsOpinion bool   // Whether to show "Opinion:" prefix
	PostCount int    // Number of posts in story
	Posts     []*RenderedPost
	IsFiller  bool              // True if this is a feed card filling a grid gap
	Filler    *RenderedFeedCard // Set when IsFiller is true - the feed card to render
}

// RenderedPost represents a post within a story, ready for templating.
type RenderedPost struct {
	// Author info
	AuthorHandle string // @handle
	AuthorName   string // Display name (escaped)
	AuthorURL    string // Profile URL

	// Content
	Text string // Post text (escaped, with newlines as <br>)

	// Metadata
	Time        string // Formatted timestamp
	URL         string // Post URL
	LikeCount   int
	RepostCount int

	// Repost info (if this is a repost)
	IsRepost       bool
	RepostByHandle string
	RepostByName   string

	// Media
	Images []RenderedImage

	// Embeds
	ExternalLink *RenderedLink
	Quote        *RenderedQuote

	// Thread context
	ThreadReplies []*RenderedPost
}

// RenderedImage represents an image ready for templating.
type RenderedImage struct {
	URL      string // Image URL
	Alt      string // Alt text (escaped)
	Thumb    string // Thumbnail URL (may be same as URL)
	GroupID  string // For lightbox grouping
	Index    int    // Index within group
}

// RenderedLink represents an external link card.
type RenderedLink struct {
	URL         string // Link URL
	Title       string // Title (escaped)
	Description string // Description (escaped, may be empty)
	Thumb       string // Thumbnail URL (may be empty)
	Domain      string // Display domain
}

// RenderedQuote represents a quoted post card.
type RenderedQuote struct {
	AuthorHandle string
	AuthorName   string
	AuthorURL    string
	Text         string // Quoted text (escaped)
	URL          string // Link to quoted post
}

// RenderedFeedCard represents a "From the Feed" content card.
type RenderedFeedCard struct {
	Post *RenderedPost
}
