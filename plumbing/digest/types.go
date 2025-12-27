package main

import "time"

// Post represents a Bluesky post in storage format (complete data)
type Post struct {
	Rkey      string     `json:"rkey"`
	URI       string     `json:"uri"`
	CID       string     `json:"cid"`
	Text      string     `json:"text"`
	Author    Author     `json:"author"`
	CreatedAt time.Time  `json:"created_at"`
	IndexedAt time.Time  `json:"indexed_at"`
	Repost    *Repost    `json:"repost,omitempty"`
	ReplyTo   *ReplyTo   `json:"reply_to,omitempty"`
	Images    []Image    `json:"images,omitempty"`
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

// DisplayPost is the minimal format for agent consumption
type DisplayPost struct {
	Rkey      string          `json:"rkey"`
	Text      string          `json:"text"`
	Author    DisplayAuthor   `json:"author"`
	CreatedAt time.Time       `json:"created_at"`
	Repost    *DisplayRepost  `json:"repost,omitempty"`
	ReplyTo   *DisplayReplyTo `json:"reply_to,omitempty"`
	Images    []Image         `json:"images,omitempty"`
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

// Summaries maps category name to summary text
type Summaries map[string]string
