package main

import "fmt"

// CategorizePosts adds posts to a category (creates if needed)
// Removes posts from their previous category (enforces one category per post)
// Returns error if any rkey doesn't exist in index
func CategorizePosts(cats Categories, index PostsIndex, category string, rkeys []string) error {
	// Validate all rkeys exist
	for _, rkey := range rkeys {
		if _, exists := index[rkey]; !exists {
			return fmt.Errorf("post not found: %s", rkey)
		}
	}

	// Remove posts from any existing categories
	for _, rkey := range rkeys {
		for catName, posts := range cats {
			// Remove rkey from this category if present
			cats[catName] = removeString(posts, rkey)
		}
	}

	// Clean up empty categories (except target)
	for catName := range cats {
		if catName != category && len(cats[catName]) == 0 {
			delete(cats, catName)
		}
	}

	// Add posts to target category (create if doesn't exist)
	if _, exists := cats[category]; !exists {
		cats[category] = []string{}
	}

	// Add each rkey if not already present (for idempotency)
	for _, rkey := range rkeys {
		if !contains(cats[category], rkey) {
			cats[category] = append(cats[category], rkey)
		}
	}

	return nil
}

// MergeCategories moves all posts from source to target
// Removes source category after merge
func MergeCategories(cats Categories, from string, to string) error {
	// Get source posts (empty if doesn't exist)
	sourcePosts, exists := cats[from]
	if !exists || len(sourcePosts) == 0 {
		// Source doesn't exist or is empty - safe no-op
		delete(cats, from) // Clean up if it exists but is empty
		return nil
	}

	// Ensure target exists
	if _, exists := cats[to]; !exists {
		cats[to] = []string{}
	}

	// Add all source posts to target (avoiding duplicates)
	for _, rkey := range sourcePosts {
		if !contains(cats[to], rkey) {
			cats[to] = append(cats[to], rkey)
		}
	}

	// Remove source category
	delete(cats, from)

	return nil
}

// GetCategoryPosts retrieves all posts in a category
// Uses index for fast lookup, returns in display format
func GetCategoryPosts(cats Categories, posts []Post, index PostsIndex, category string) ([]DisplayPost, error) {
	rkeys, exists := cats[category]
	if !exists || len(rkeys) == 0 {
		return []DisplayPost{}, nil
	}

	// Lookup posts by index
	categoryPosts := make([]Post, 0, len(rkeys))
	for _, rkey := range rkeys {
		idx, exists := index[rkey]
		if !exists {
			// Post not found in index - skip it (shouldn't happen, but be safe)
			continue
		}
		if idx < len(posts) {
			categoryPosts = append(categoryPosts, posts[idx])
		}
	}

	// Convert to display format
	return FormatForDisplay(categoryPosts), nil
}

// ListCategoriesWithCounts returns map of category -> count
func ListCategoriesWithCounts(cats Categories) map[string]int {
	counts := make(map[string]int, len(cats))
	for catName, posts := range cats {
		counts[catName] = len(posts)
	}
	return counts
}

// GetUncategorizedPosts finds posts not in any category
func GetUncategorizedPosts(cats Categories, index PostsIndex) []string {
	// Build set of all categorized rkeys
	categorized := make(map[string]bool)
	for _, posts := range cats {
		for _, rkey := range posts {
			categorized[rkey] = true
		}
	}

	// Find rkeys in index but not categorized
	uncategorized := []string{}
	for rkey := range index {
		if !categorized[rkey] {
			uncategorized = append(uncategorized, rkey)
		}
	}

	return uncategorized
}

// Helper: remove string from slice
func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// Helper: check if slice contains string
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
