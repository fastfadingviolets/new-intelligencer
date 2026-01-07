package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WorkspaceData holds all data loaded from a workspace
type WorkspaceData struct {
	Dir           string
	Config        Config
	Posts         []Post
	Index         PostsIndex
	Categories    Categories
	ThreadReplies map[string][]string // parentRkey → []childRkeys (built lazily)
}

// extractRkeyFromURI extracts the rkey from an AT URI like "at://did:plc:xyz/app.bsky.feed.post/rkey123"
func extractRkeyFromURI(uri string) string {
	parts := strings.Split(uri, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// BuildThreadGraph builds the thread reply map from posts
func (wd *WorkspaceData) BuildThreadGraph() {
	wd.ThreadReplies = make(map[string][]string)
	for _, post := range wd.Posts {
		if post.ReplyTo != nil && post.ReplyTo.URI != "" {
			parentRkey := extractRkeyFromURI(post.ReplyTo.URI)
			if parentRkey != "" {
				wd.ThreadReplies[parentRkey] = append(wd.ThreadReplies[parentRkey], post.Rkey)
			}
		}
	}
}

// IsReply returns true if the post is a reply to another post
func (wd *WorkspaceData) IsReply(rkey string) bool {
	if idx, ok := wd.Index[rkey]; ok && idx < len(wd.Posts) {
		return wd.Posts[idx].ReplyTo != nil
	}
	return false
}

// GetReplies returns all reply rkeys for a given parent post rkey
func (wd *WorkspaceData) GetReplies(parentRkey string) []string {
	if wd.ThreadReplies == nil {
		wd.BuildThreadGraph()
	}
	return wd.ThreadReplies[parentRkey]
}

// GetWorkspaceDir returns the workspace directory path
// Uses --dir flag if set, otherwise finds the most recent digest-* in current directory
func GetWorkspaceDir() (string, error) {
	if workspaceDir != "" {
		return workspaceDir, nil
	}

	// Look for digest-* directories in current directory
	entries, err := os.ReadDir(".")
	if err != nil {
		return "", fmt.Errorf("reading current directory: %w", err)
	}

	var newest string
	var newestDate time.Time

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "digest-") {
			// Parse date from folder name (format: digest-DD-MM-YYYY)
			dateStr := strings.TrimPrefix(entry.Name(), "digest-")
			date, err := time.Parse("02-01-2006", dateStr)
			if err != nil {
				// Can't parse date, skip this folder
				continue
			}
			if newest == "" || date.After(newestDate) {
				newest = entry.Name()
				newestDate = date
			}
		}
	}

	if newest == "" {
		return "", fmt.Errorf("no workspace found - run 'digest init' first or use --dir flag")
	}

	return newest, nil
}

// GenerateWorkspaceDir creates a workspace directory name from current date
func GenerateWorkspaceDir(since time.Time) string {
	// Format as digest-DD-MM-YYYY
	return since.Format("digest-02-01-2006")
}

// LoadWorkspace loads all data from workspace directory
func LoadWorkspace(dir string) (*WorkspaceData, error) {
	// Verify directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace not found: %s", dir)
	}

	wd := &WorkspaceData{Dir: dir}

	// Load config
	configPath := filepath.Join(dir, "config.json")
	if _, err := os.Stat(configPath); err == nil {
		var err error
		// For now, we'll skip loading config since we don't have a LoadConfig function yet
		// This can be added when needed
		_ = err
	}

	// Load posts
	postsPath := filepath.Join(dir, "posts.json")
	posts, err := LoadPosts(postsPath)
	if err != nil {
		return nil, fmt.Errorf("loading posts: %w", err)
	}
	wd.Posts = posts

	// Load index
	indexPath := filepath.Join(dir, "posts-index.json")
	index, err := LoadIndex(indexPath)
	if err != nil {
		return nil, fmt.Errorf("loading index: %w", err)
	}
	wd.Index = index

	// Load categories
	catsPath := filepath.Join(dir, "categories.json")
	cats, err := LoadCategories(catsPath)
	if err != nil {
		return nil, fmt.Errorf("loading categories: %w", err)
	}
	wd.Categories = cats

	return wd, nil
}

// SaveWorkspaceData saves updated data back to workspace
func SaveWorkspaceData(wd *WorkspaceData) error {
	if err := SavePosts(filepath.Join(wd.Dir, "posts.json"), wd.Posts); err != nil {
		return fmt.Errorf("saving posts: %w", err)
	}

	if err := SaveIndex(filepath.Join(wd.Dir, "posts-index.json"), wd.Index); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}

	if err := SaveCategories(filepath.Join(wd.Dir, "categories.json"), wd.Categories); err != nil {
		return fmt.Errorf("saving categories: %w", err)
	}

	return nil
}
