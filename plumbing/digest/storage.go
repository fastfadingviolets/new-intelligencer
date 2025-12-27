package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExtractRkey extracts the rkey (record key) from an AT Protocol URI
// Example: "at://did:plc:xyz/app.bsky.feed.post/3lbkj2x3abcd" → "3lbkj2x3abcd"
func ExtractRkey(uri string) string {
	parts := strings.Split(uri, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// LoadPosts reads posts from a JSON file
// Returns empty array if file doesn't exist
func LoadPosts(path string) ([]Post, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Post{}, nil
		}
		return nil, fmt.Errorf("reading posts file: %w", err)
	}

	var posts []Post
	if err := json.Unmarshal(data, &posts); err != nil {
		return nil, fmt.Errorf("parsing posts JSON: %w", err)
	}

	return posts, nil
}

// SavePosts writes posts to a JSON file atomically
// Uses temp file + rename pattern for atomic writes
func SavePosts(path string, posts []Post) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling posts: %w", err)
	}

	// Write to temp file
	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, path); err != nil {
		os.Remove(tempFile) // Clean up temp file on error
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// LoadCategories reads categories from a JSON file
// Returns empty map if file doesn't exist
// Supports backward compatibility with old format (map[string][]string)
func LoadCategories(path string) (Categories, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Categories{}, nil
		}
		return nil, fmt.Errorf("reading categories file: %w", err)
	}

	// Try new format first (map[string]CategoryData)
	var cats Categories
	if err := json.Unmarshal(data, &cats); err == nil {
		if cats == nil {
			cats = Categories{}
		}
		return cats, nil
	}

	// Fall back to old format (map[string][]string) and migrate
	var oldCats map[string][]string
	if err := json.Unmarshal(data, &oldCats); err != nil {
		return nil, fmt.Errorf("parsing categories JSON (tried both formats): %w", err)
	}

	// Migrate old format to new format
	cats = make(Categories, len(oldCats))
	for catName, rkeys := range oldCats {
		cats[catName] = CategoryData{
			Visible: rkeys,
			Hidden:  []string{},
		}
	}

	return cats, nil
}

// SaveCategories writes categories to a JSON file atomically
func SaveCategories(path string, cats Categories) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(cats, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling categories: %w", err)
	}

	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tempFile, path); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// LoadSummaries reads summaries from a JSON file
// Returns empty map if file doesn't exist
func LoadSummaries(path string) (Summaries, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Summaries{}, nil
		}
		return nil, fmt.Errorf("reading summaries file: %w", err)
	}

	var sums Summaries
	if err := json.Unmarshal(data, &sums); err != nil {
		return nil, fmt.Errorf("parsing summaries JSON: %w", err)
	}

	if sums == nil {
		sums = Summaries{}
	}

	return sums, nil
}

// SaveSummaries writes summaries to a JSON file atomically
func SaveSummaries(path string, sums Summaries) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(sums, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling summaries: %w", err)
	}

	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tempFile, path); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// LoadIndex reads posts index from a JSON file
// Returns empty map if file doesn't exist
func LoadIndex(path string) (PostsIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PostsIndex{}, nil
		}
		return nil, fmt.Errorf("reading index file: %w", err)
	}

	var index PostsIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parsing index JSON: %w", err)
	}

	if index == nil {
		index = PostsIndex{}
	}

	return index, nil
}

// SaveIndex writes posts index to a JSON file atomically
func SaveIndex(path string, index PostsIndex) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling index: %w", err)
	}

	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tempFile, path); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// BuildIndex creates an index mapping rkey to array index
func BuildIndex(posts []Post) PostsIndex {
	index := make(PostsIndex, len(posts))
	for i, post := range posts {
		index[post.Rkey] = i
	}
	return index
}
