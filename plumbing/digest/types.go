package main

import "time"

// Post represents a Bluesky post in storage format (complete data)
type Post struct {
	Rkey         string        `json:"rkey"`
	URI          string        `json:"uri"`
	CID          string        `json:"cid"`
	Text         string        `json:"text"`
	Author       Author        `json:"author"`
	CreatedAt    time.Time     `json:"created_at"`
	IndexedAt    time.Time     `json:"indexed_at"`
	Repost       *Repost       `json:"repost,omitempty"`
	ReplyTo      *ReplyTo      `json:"reply_to,omitempty"`
	Images       []Image       `json:"images,omitempty"`
	ExternalLink *ExternalLink `json:"external_link,omitempty"`
	LikeCount    int64         `json:"like_count"`
	ReplyCount   int64         `json:"reply_count"`
	RepostCount  int64         `json:"repost_count"`
	QuoteCount   int64         `json:"quote_count"`
}

// Author represents post author information
type Author struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name,omitempty"`
}

// Repost holds metadata when a post is a repost
type Repost struct {
	ByDID    string    `json:"by_did"`
	ByHandle string    `json:"by_handle"`
	At       time.Time `json:"at"`
}

// ReplyTo holds metadata when a post is a reply
type ReplyTo struct {
	URI          string `json:"uri"`
	AuthorHandle string `json:"author_handle"`
}

// Image represents an image attachment
type Image struct {
	URL string `json:"url"`
	Alt string `json:"alt,omitempty"`
}

// ExternalLink represents an embedded link preview (article, website, etc.)
type ExternalLink struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Thumb       string `json:"thumb,omitempty"`
}

// DisplayPost is the minimal format for agent consumption
type DisplayPost struct {
	Rkey        string          `json:"rkey"`
	Text        string          `json:"text"`
	Author      DisplayAuthor   `json:"author"`
	CreatedAt   time.Time       `json:"created_at"`
	Repost      *DisplayRepost  `json:"repost,omitempty"`
	ReplyTo      *DisplayReplyTo `json:"reply_to,omitempty"`
	Images       []Image         `json:"images,omitempty"`
	ExternalLink *ExternalLink   `json:"external_link,omitempty"`
	LikeCount    int64           `json:"like_count"`
	ReplyCount  int64           `json:"reply_count"`
	RepostCount int64           `json:"repost_count"`
	QuoteCount  int64           `json:"quote_count"`
}

// DisplayAuthor is author info without DID
type DisplayAuthor struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name,omitempty"`
}

// DisplayRepost is repost metadata without DID
type DisplayRepost struct {
	ByHandle string    `json:"by_handle"`
	At       time.Time `json:"at"`
}

// DisplayReplyTo is reply metadata without URI
type DisplayReplyTo struct {
	AuthorHandle string `json:"author_handle"`
}

// Config holds workspace configuration
type Config struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	TimeRange TimeRange `json:"time_range"`
}

// TimeRange specifies the time window for fetching posts
type TimeRange struct {
	Since time.Time  `json:"since"`
	Until *time.Time `json:"until,omitempty"`
}

// PostsIndex maps rkey to array index for fast lookup
type PostsIndex map[string]int

// CategoryData holds posts for a category, split into visible and hidden
type CategoryData struct {
	Visible      []string `json:"visible"`
	Hidden       []string `json:"hidden,omitempty"`
	HiddenReason string   `json:"hiddenReason,omitempty"`
	IsHidden     bool     `json:"isHidden,omitempty"`
}

// Categories maps category name to category data
type Categories map[string]CategoryData

// NewspaperSection defines a section in the newspaper (from project-level config)
type NewspaperSection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`                  // "news" or "content"
	Description string `json:"description,omitempty"` // Helps agent understand what belongs here
	MaxStories  int    `json:"max_stories,omitempty"` // Max stories to show in compiled output
}

// NewspaperConfig is the project-level newspaper structure (lives at project root)
type NewspaperConfig struct {
	Sections []NewspaperSection `json:"sections"`
}

// StoryGroup represents consolidated posts about same news story (created by agent)
type StoryGroup struct {
	ID          string   `json:"id"`
	Headline    string   `json:"headline"`
	Summary     string   `json:"summary,omitempty"`
	ArticleURL  string   `json:"article_url,omitempty"`
	PostRkeys   []string `json:"post_rkeys"`
	PrimaryRkey string   `json:"primary_rkey"`
	IsOpinion   bool     `json:"is_opinion"`
	SectionID   string   `json:"section_id"`
	Role        string   `json:"role"`       // "headline", "featured", "opinion"
	IsFrontPage bool     `json:"front_page"` // if true, appears on front page only
	Priority    int      `json:"priority"`   // 1 = most important, higher = less important
}

// StoryGroups maps story group ID to story group data
type StoryGroups map[string]StoryGroup

// ContentPicks tracks Claude's picks for content sections
type ContentPicks struct {
	SectionID  string   `json:"section_id"`
	SuiGeneris []string `json:"sui_generis"` // rkeys Claude finds interesting
}

// AllContentPicks maps section ID to content picks
type AllContentPicks map[string]ContentPicks
