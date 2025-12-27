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

	// Remove posts from any existing categories (both visible and hidden)
	for _, rkey := range rkeys {
		for catName, catData := range cats {
			catData.Visible = removeString(catData.Visible, rkey)
			catData.Hidden = removeString(catData.Hidden, rkey)
			cats[catName] = catData
		}
	}

	// Clean up empty categories (except target)
	for catName, catData := range cats {
		if catName != category && len(catData.Visible) == 0 && len(catData.Hidden) == 0 {
			delete(cats, catName)
		}
	}

	// Add posts to target category (create if doesn't exist)
	catData, exists := cats[category]
	if !exists {
		catData = CategoryData{
			Visible: []string{},
			Hidden:  []string{},
		}
	}

	// Add each rkey if not already present (for idempotency)
	for _, rkey := range rkeys {
		if !contains(catData.Visible, rkey) {
			catData.Visible = append(catData.Visible, rkey)
		}
	}

	cats[category] = catData
	return nil
}

// MergeCategories moves all posts from source to target
// Removes source category after merge
// Merges both visible and hidden posts, combining hidden reasons
func MergeCategories(cats Categories, from string, to string) error {
	// Get source category data
	sourceData, exists := cats[from]
	if !exists || (len(sourceData.Visible) == 0 && len(sourceData.Hidden) == 0) {
		// Source doesn't exist or is empty - safe no-op
		delete(cats, from) // Clean up if it exists but is empty
		return nil
	}

	// Ensure target exists
	targetData, exists := cats[to]
	if !exists {
		targetData = CategoryData{
			Visible: []string{},
			Hidden:  []string{},
		}
	}

	// Merge visible posts (avoiding duplicates)
	for _, rkey := range sourceData.Visible {
		if !contains(targetData.Visible, rkey) && !contains(targetData.Hidden, rkey) {
			targetData.Visible = append(targetData.Visible, rkey)
		}
	}

	// Merge hidden posts (avoiding duplicates)
	for _, rkey := range sourceData.Hidden {
		if !contains(targetData.Visible, rkey) && !contains(targetData.Hidden, rkey) {
			targetData.Hidden = append(targetData.Hidden, rkey)
		}
	}

	// Merge hidden reasons if both have them
	if sourceData.HiddenReason != "" {
		if targetData.HiddenReason != "" && targetData.HiddenReason != sourceData.HiddenReason {
			targetData.HiddenReason = targetData.HiddenReason + "; " + sourceData.HiddenReason
		} else if targetData.HiddenReason == "" {
			targetData.HiddenReason = sourceData.HiddenReason
		}
	}

	cats[to] = targetData

	// Remove source category
	delete(cats, from)

	return nil
}

// HideCategory marks a category as hidden so it won't appear in the digest
// Returns the number of posts in the category (visible + hidden)
// Returns error if category doesn't exist or is already hidden
func HideCategory(cats Categories, category string) (int, error) {
	catData, exists := cats[category]
	if !exists {
		return 0, fmt.Errorf("category '%s' not found", category)
	}

	if catData.IsHidden {
		return 0, fmt.Errorf("category '%s' is already hidden", category)
	}

	postCount := len(catData.Visible) + len(catData.Hidden)
	catData.IsHidden = true
	cats[category] = catData

	return postCount, nil
}

// UnhideCategory marks a hidden category as visible again
// Returns error if category doesn't exist or is not hidden
func UnhideCategory(cats Categories, category string) error {
	catData, exists := cats[category]
	if !exists {
		return fmt.Errorf("category '%s' not found", category)
	}

	if !catData.IsHidden {
		return fmt.Errorf("category '%s' is not hidden", category)
	}

	catData.IsHidden = false
	cats[category] = catData

	return nil
}

// GetCategoryPosts retrieves all visible posts in a category (excludes hidden)
// Uses index for fast lookup, returns in display format
func GetCategoryPosts(cats Categories, posts []Post, index PostsIndex, category string) ([]DisplayPost, error) {
	catData, exists := cats[category]
	if !exists || len(catData.Visible) == 0 {
		return []DisplayPost{}, nil
	}

	// Lookup posts by index (only visible posts)
	categoryPosts := make([]Post, 0, len(catData.Visible))
	for _, rkey := range catData.Visible {
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

// ListCategoriesWithCounts returns map of category -> visible post count
func ListCategoriesWithCounts(cats Categories) map[string]int {
	counts := make(map[string]int, len(cats))
	for catName, catData := range cats {
		counts[catName] = len(catData.Visible)
	}
	return counts
}

// GetUncategorizedPosts finds posts not in any category (both visible and hidden count as categorized)
func GetUncategorizedPosts(cats Categories, index PostsIndex) []string {
	// Build set of all categorized rkeys (both visible and hidden)
	categorized := make(map[string]bool)
	for _, catData := range cats {
		for _, rkey := range catData.Visible {
			categorized[rkey] = true
		}
		for _, rkey := range catData.Hidden {
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

// HidePosts moves posts from visible to hidden within a category
// Returns error if category doesn't exist or if any rkey is not in category's visible posts
func HidePosts(cats Categories, category string, rkeys []string, reason string) error {
	catData, exists := cats[category]
	if !exists {
		return fmt.Errorf("category '%s' not found", category)
	}

	// Validate all rkeys are in visible posts
	for _, rkey := range rkeys {
		if !contains(catData.Visible, rkey) {
			return fmt.Errorf("post '%s' not found in category '%s' visible posts", rkey, category)
		}
	}

	// Move posts from visible to hidden
	for _, rkey := range rkeys {
		catData.Visible = removeString(catData.Visible, rkey)
		if !contains(catData.Hidden, rkey) {
			catData.Hidden = append(catData.Hidden, rkey)
		}
	}

	// Set or append reason
	if catData.HiddenReason == "" {
		catData.HiddenReason = reason
	} else if reason != "" && catData.HiddenReason != reason {
		// Only append if different reason provided
		catData.HiddenReason = catData.HiddenReason + "; " + reason
	}

	cats[category] = catData
	return nil
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
