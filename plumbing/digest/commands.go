package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gofrs/flock"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
)

// withLock executes fn while holding an exclusive lock on lockFile
func withLock(dir, lockFile string, fn func() error) error {
	lockPath := filepath.Join(dir, lockFile)
	fileLock := flock.New(lockPath)
	if err := fileLock.Lock(); err != nil {
		return err
	}
	defer fileLock.Unlock()
	return fn()
}

// findPostCategory returns the category ID a post is in, or empty string if not found
func findPostCategory(rkey string, cats Categories) string {
	for catID, cat := range cats {
		for _, r := range cat.Visible {
			if r == rkey {
				return catID
			}
		}
		for _, r := range cat.Hidden {
			if r == rkey {
				return catID
			}
		}
	}
	return ""
}

// removeFromCategory removes a post from its current category
func removeFromCategory(rkey string, cats Categories) {
	for catID, cat := range cats {
		// Remove from Visible
		newVisible := []string{}
		for _, r := range cat.Visible {
			if r != rkey {
				newVisible = append(newVisible, r)
			}
		}
		// Remove from Hidden
		newHidden := []string{}
		for _, r := range cat.Hidden {
			if r != rkey {
				newHidden = append(newHidden, r)
			}
		}
		if len(newVisible) != len(cat.Visible) || len(newHidden) != len(cat.Hidden) {
			cat.Visible = newVisible
			cat.Hidden = newHidden
			cats[catID] = cat
			return
		}
	}
}

// Config for environment variables (Bluesky credentials)
type EnvConfig struct {
	Handle   string `envconfig:"BSKY_HANDLE" required:"true"`
	Password string `envconfig:"BSKY_PASSWORD" required:"true"`
	PDSHost  string `envconfig:"BSKY_PDS_HOST" default:"https://bsky.social"`
}

// digest init
var initCmd = &cobra.Command{
	Use:   "init [--since TIMESTAMP]",
	Short: "Initialize a new digest workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		sinceStr, _ := cmd.Flags().GetString("since")

		var since time.Time
		if sinceStr != "" {
			var err error
			since, err = time.Parse(time.RFC3339, sinceStr)
			if err != nil {
				return fmt.Errorf("invalid --since timestamp: %w", err)
			}
		} else {
			since = time.Now().Add(-24 * time.Hour)
		}

		// Generate workspace directory name
		dirName := GenerateWorkspaceDir(since)

		// Create directory
		if err := os.MkdirAll(dirName, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}

		// Initialize empty files
		config := Config{
			Version:   "1",
			CreatedAt: time.Now(),
			TimeRange: TimeRange{Since: since},
		}

		// Save config
		configData, _ := json.MarshalIndent(config, "", "  ")
		os.WriteFile(filepath.Join(dirName, "config.json"), configData, 0644)

		// Save empty data files
		SavePosts(filepath.Join(dirName, "posts.json"), []Post{})
		SaveIndex(filepath.Join(dirName, "posts-index.json"), PostsIndex{})
		SaveCategories(filepath.Join(dirName, "categories.json"), Categories{})

		fmt.Printf("Initialized workspace: %s\n", dirName)
		fmt.Printf("Time range: %s onwards\n", since.Format("2006-01-02 15:04"))
		return nil
	},
}

// digest fetch
var fetchCmd = &cobra.Command{
	Use:   "fetch [--limit N]",
	Short: "Fetch posts from Bluesky timeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")

		// Get workspace
		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		// Load config
		var envCfg EnvConfig
		if err := envconfig.Process("", &envCfg); err != nil {
			return fmt.Errorf("loading credentials from environment: %w", err)
		}

		// Authenticate
		fmt.Printf("Authenticating with %s...\n", envCfg.PDSHost)
		client, err := Authenticate(envCfg.Handle, envCfg.Password, envCfg.PDSHost)
		if err != nil {
			return err
		}

		// Fetch posts
		fmt.Println("Fetching posts...")
		since := time.Now().Add(-24 * time.Hour) // TODO: Load from config
		result, err := FetchPosts(client, since, limit)
		if err != nil {
			return err
		}

		// Save to workspace
		if err := SavePosts(filepath.Join(dir, "posts.json"), result.Posts); err != nil {
			return err
		}
		if err := SaveIndex(filepath.Join(dir, "posts-index.json"), result.Index); err != nil {
			return err
		}

		fmt.Printf("Fetched %d posts to %s/\n", result.Total, dir)
		return nil
	},
}

// digest read-posts
var readPostsCmd = &cobra.Command{
	Use:   "read-posts",
	Short: "Display posts from workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		offset, _ := cmd.Flags().GetInt("offset")
		limit, _ := cmd.Flags().GetInt("limit")

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		wd, err := LoadWorkspace(dir)
		if err != nil {
			return err
		}
		wd.BuildThreadGraph()

		// Build set of rkeys that are replies to posts we have
		repliesInDataset := make(map[string]bool)
		for _, post := range wd.Posts {
			if post.ReplyTo != nil {
				parentRkey := extractRkeyFromURI(post.ReplyTo.URI)
				// Check if parent is in our dataset
				if _, ok := wd.Index[parentRkey]; ok {
					repliesInDataset[post.Rkey] = true
				}
			}
		}

		// Filter to only thread roots (non-replies or replies to posts not in dataset)
		var roots []Post
		for _, post := range wd.Posts {
			if !repliesInDataset[post.Rkey] {
				roots = append(roots, post)
			}
		}

		// Apply offset and limit to roots
		end := len(roots)
		if offset >= end {
			offset = end
		}
		if limit > 0 && offset+limit < end {
			end = offset + limit
		}

		roots = roots[offset:end]

		// Format for display with thread info
		displayPosts := FormatForDisplayWithThreads(roots, wd)

		// Fetch and encode first image for each post (only if --images flag is set)
		if includeImages, _ := cmd.Flags().GetBool("images"); includeImages {
			FetchImagesForDisplay(displayPosts)
		}

		data, _ := json.MarshalIndent(displayPosts, "", "  ")
		fmt.Println(string(data))

		return nil
	},
}

// digest categorize
var categorizeCmd = &cobra.Command{
	Use:   "categorize <category> <rkey>...",
	Short: "Assign posts to a category",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		category := args[0]
		rkeys := args[1:]
		moveFlag, _ := cmd.Flags().GetBool("move")

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		return categorizeInDir(dir, category, rkeys, moveFlag)
	},
}

// categorizeInDir categorizes posts in the given workspace directory
func categorizeInDir(dir, category string, rkeys []string, moveFlag bool) error {
	return withLock(dir, "categories.lock", func() error {
		// Load workspace AFTER acquiring lock (data may have changed)
		wd, err := LoadWorkspace(dir)
		if err != nil {
			return err
		}

		// Build thread graph to auto-include replies
		wd.BuildThreadGraph()

		// Expand rkeys to include thread replies
		expandedRkeys := []string{}
		for _, rkey := range rkeys {
			expandedRkeys = append(expandedRkeys, rkey)
			// Auto-include any replies to this post
			for _, replyRkey := range wd.GetReplies(rkey) {
				expandedRkeys = append(expandedRkeys, replyRkey)
			}
		}
		rkeys = expandedRkeys

		// Process rkeys based on --move flag
		newRkeys := []string{}
		skippedRkeys := []string{}
		movedRkeys := []string{}

		for _, rkey := range rkeys {
			existingCat := findPostCategory(rkey, wd.Categories)
			if existingCat != "" {
				if moveFlag {
					// Remove from old category, will add to new
					removeFromCategory(rkey, wd.Categories)
					newRkeys = append(newRkeys, rkey)
					movedRkeys = append(movedRkeys, rkey)
				} else {
					// Skip already-categorized (first-claim wins)
					skippedRkeys = append(skippedRkeys, rkey)
				}
			} else {
				newRkeys = append(newRkeys, rkey)
			}
		}

		// Only categorize if we have posts to add
		if len(newRkeys) > 0 {
			if err := CategorizePosts(wd.Categories, wd.Index, category, newRkeys); err != nil {
				return err
			}

			// Save
			if err := SaveCategories(filepath.Join(wd.Dir, "categories.json"), wd.Categories); err != nil {
				return err
			}
		}

		// Report results
		if len(movedRkeys) > 0 {
			fmt.Printf("Moved %d posts to '%s'\n", len(movedRkeys), category)
		} else if len(skippedRkeys) > 0 {
			fmt.Printf("Categorized %d posts into '%s' (skipped %d already-categorized)\n",
				len(newRkeys), category, len(skippedRkeys))
		} else {
			fmt.Printf("Categorized %d posts into '%s'\n", len(newRkeys), category)
		}
		return nil
	})
}

// digest list-categories
var listCategoriesCmd = &cobra.Command{
	Use:   "list-categories",
	Short: "List all categories",
	RunE: func(cmd *cobra.Command, args []string) error {
		withCounts, _ := cmd.Flags().GetBool("with-counts")

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		cats, err := LoadCategories(filepath.Join(dir, "categories.json"))
		if err != nil {
			return err
		}

		// Filter out hidden categories
		visibleCats := make(Categories)
		for name, catData := range cats {
			if !catData.IsHidden {
				visibleCats[name] = catData
			}
		}

		if withCounts {
			counts := ListCategoriesWithCounts(visibleCats)
			for cat, count := range counts {
				fmt.Printf("%s (%d posts)\n", cat, count)
			}
		} else {
			for cat := range visibleCats {
				fmt.Println(cat)
			}
		}

		return nil
	},
}

// digest show-category
var showCategoryCmd = &cobra.Command{
	Use:   "show-category <category>",
	Short: "Display posts in a category",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		category := args[0]

		wd, err := LoadWorkspace(workspaceDir)
		if err != nil {
			dir, _ := GetWorkspaceDir()
			wd, err = LoadWorkspace(dir)
			if err != nil {
				return err
			}
		}

		displayPosts, err := GetCategoryPosts(wd.Categories, wd.Posts, wd.Index, category)
		if err != nil {
			return err
		}

		data, _ := json.MarshalIndent(displayPosts, "", "  ")
		fmt.Println(string(data))

		return nil
	},
}

// digest compile
var compileCmd = &cobra.Command{
	Use:   "compile",
	Short: "Generate markdown and HTML digest",
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")

		wd, err := LoadWorkspace(workspaceDir)
		if err != nil {
			dir, _ := GetWorkspaceDir()
			wd, err = LoadWorkspace(dir)
			if err != nil {
				return err
			}
		}

		config := Config{CreatedAt: time.Now()} // TODO: Load from file

		// Load newspaper config from workspace's parent directory (project root)
		newspaperPath := filepath.Join(filepath.Dir(wd.Dir), "newspaper.json")
		newspaperConfig, err := LoadNewspaperConfig(newspaperPath)
		if err != nil {
			return fmt.Errorf("loading %s: %w", newspaperPath, err)
		}

		// Load workspace-specific data
		storyGroups, err := LoadStoryGroups(filepath.Join(wd.Dir, "story-groups.json"))
		if err != nil {
			return err
		}
		contentPicks, err := LoadContentPicks(filepath.Join(wd.Dir, "content-picks.json"))
		if err != nil {
			return err
		}

		// Validate all stories have headline and priority
		var unprocessed []string
		for id, story := range storyGroups {
			if story.Headline == "" || story.Priority == 0 {
				missing := []string{}
				if story.Headline == "" {
					missing = append(missing, "headline")
				}
				if story.Priority == 0 {
					missing = append(missing, "priority")
				}
				unprocessed = append(unprocessed, fmt.Sprintf("  %s [%s] (missing: %s)", id, story.SectionID, joinStrings(missing, ", ")))
			}
		}
		if len(unprocessed) > 0 {
			sort.Strings(unprocessed)
			return fmt.Errorf("compile blocked: %d stories are unprocessed\n%s\n\nRun `./bin/digest show-unprocessed` for details",
				len(unprocessed), joinStrings(unprocessed, "\n"))
		}

		// Validate front page: exactly one headline, and it must not be opinion
		var frontPageHeadlines []string
		for id := range storyGroups {
			story := storyGroups[id]
			if story.SectionID == "front-page" && story.Role == "headline" {
				frontPageHeadlines = append(frontPageHeadlines, fmt.Sprintf("%s: %s", id, story.Headline))
				if story.IsOpinion {
					return fmt.Errorf("front page headline cannot be an opinion piece: %s", id)
				}
			}
		}
		if len(frontPageHeadlines) > 1 {
			return fmt.Errorf("multiple front-page headlines found (only one allowed):\n  - %s",
				joinStrings(frontPageHeadlines, "\n  - "))
		}

		// Generate markdown digest
		markdown, err := CompileDigest(wd.Posts, wd.Categories, storyGroups, newspaperConfig, contentPicks, config)
		if err != nil {
			return err
		}

		// Generate HTML digest
		htmlContent, err := CompileDigestHTML(wd.Posts, wd.Categories, storyGroups, newspaperConfig, contentPicks, config)
		if err != nil {
			return err
		}

		// Write markdown
		mdOutput := output
		if mdOutput == "" {
			mdOutput = filepath.Join(wd.Dir, "digest.md")
		}
		if err := os.WriteFile(mdOutput, []byte(markdown), 0644); err != nil {
			return err
		}
		fmt.Printf("Compiled markdown digest to %s\n", mdOutput)

		// Write HTML
		htmlOutput := filepath.Join(wd.Dir, "digest.html")
		if err := os.WriteFile(htmlOutput, []byte(htmlContent), 0644); err != nil {
			return err
		}
		fmt.Printf("Compiled HTML digest to %s\n", htmlOutput)

		return nil
	},
}

// digest status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show workspace status",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := LoadWorkspace(workspaceDir)
		if err != nil {
			dir, _ := GetWorkspaceDir()
			wd, err = LoadWorkspace(dir)
			if err != nil {
				return err
			}
		}

		fmt.Printf("Digest: %s/\n\n", wd.Dir)
		fmt.Printf("Posts: %d total\n", len(wd.Posts))

		uncategorized := GetUncategorizedPosts(wd.Categories, wd.Index)
		categorized := len(wd.Posts) - len(uncategorized)
		fmt.Printf("  Categorized: %d\n", categorized)
		fmt.Printf("  Uncategorized: %d\n\n", len(uncategorized))

		// Show batch progress
		bp, _ := loadBatchProgress(wd.Dir)
		if bp != nil {
			// Categorization progress
			expectedCatBatches := (len(wd.Posts) + 99) / 100 // ceil(posts/100)
			completedCatBatches := len(bp.Categorization)
			if completedCatBatches > 0 || expectedCatBatches > 0 {
				if completedCatBatches >= expectedCatBatches {
					fmt.Printf("Categorization: %d/%d batches complete\n", completedCatBatches, expectedCatBatches)
				} else {
					fmt.Printf("Categorization: %d/%d batches complete (in progress)\n", completedCatBatches, expectedCatBatches)
				}
			}

			// Consolidation progress - sections with posts (non-front-page)
			sectionsWithPosts := []string{}
			for cat := range wd.Categories {
				if cat != "front-page" {
					catData := wd.Categories[cat]
					if len(catData.Visible) > 0 || len(catData.Hidden) > 0 {
						sectionsWithPosts = append(sectionsWithPosts, cat)
					}
				}
			}
			if len(sectionsWithPosts) > 0 {
				completedConsolidation := len(bp.Consolidation)
				missing := []string{}
				for _, s := range sectionsWithPosts {
					found := false
					for _, c := range bp.Consolidation {
						if c == s {
							found = true
							break
						}
					}
					if !found {
						missing = append(missing, s)
					}
				}
				if len(missing) == 0 {
					fmt.Printf("Consolidation: %d/%d sections complete\n", completedConsolidation, len(sectionsWithPosts))
				} else {
					fmt.Printf("Consolidation: %d/%d sections complete (missing: %s)\n", completedConsolidation, len(sectionsWithPosts), joinStrings(missing, ", "))
				}
			}

			// Headlines progress - sections with stories
			storyGroups, _ := LoadStoryGroups(filepath.Join(wd.Dir, "story-groups.json"))
			sectionsWithStories := make(map[string]bool)
			for _, story := range storyGroups {
				sectionsWithStories[story.SectionID] = true
			}
			if len(sectionsWithStories) > 0 {
				completedHeadlines := len(bp.Headlines)
				missing := []string{}
				for s := range sectionsWithStories {
					found := false
					for _, h := range bp.Headlines {
						if h == s {
							found = true
							break
						}
					}
					if !found {
						missing = append(missing, s)
					}
				}
				if len(missing) == 0 {
					fmt.Printf("Headlines: %d/%d sections complete\n", completedHeadlines, len(sectionsWithStories))
				} else {
					fmt.Printf("Headlines: %d/%d sections complete (missing: %s)\n", completedHeadlines, len(sectionsWithStories), joinStrings(missing, ", "))
				}
			}

			fmt.Println()
		}

		// Separate visible and hidden categories
		visibleCats := make(map[string]int)
		hiddenCats := make(map[string]int)
		for cat, catData := range wd.Categories {
			count := len(catData.Visible)
			if catData.IsHidden {
				hiddenCats[cat] = count
			} else if count > 0 {
				visibleCats[cat] = count
			}
		}

		fmt.Printf("Categories: %d visible, %d hidden\n", len(visibleCats), len(hiddenCats))
		for cat, count := range visibleCats {
			catData := wd.Categories[cat]
			hiddenPostCount := len(catData.Hidden)

			if hiddenPostCount > 0 {
				fmt.Printf("  %s: %d visible, %d hidden\n", cat, count, hiddenPostCount)
			} else {
				fmt.Printf("  %s: %d posts\n", cat, count)
			}
		}

		// Show hidden categories
		if len(hiddenCats) > 0 {
			fmt.Printf("\nHidden categories:\n")
			for cat, count := range hiddenCats {
				catData := wd.Categories[cat]
				totalPosts := count + len(catData.Hidden)
				fmt.Printf("  [hidden] %s: %d posts\n", cat, totalPosts)
			}
		}

		return nil
	},
}

// digest uncategorized
var uncategorizedCmd = &cobra.Command{
	Use:   "uncategorized",
	Short: "Show uncategorized posts",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := LoadWorkspace(workspaceDir)
		if err != nil {
			dir, _ := GetWorkspaceDir()
			wd, err = LoadWorkspace(dir)
			if err != nil {
				return err
			}
		}

		rkeys := GetUncategorizedPosts(wd.Categories, wd.Index)

		// Get full posts
		posts := []Post{}
		for _, rkey := range rkeys {
			if idx, ok := wd.Index[rkey]; ok && idx < len(wd.Posts) {
				posts = append(posts, wd.Posts[idx])
			}
		}

		displayPosts := FormatForDisplay(posts)

		data, _ := json.MarshalIndent(displayPosts, "", "  ")
		fmt.Println(string(data))

		return nil
	},
}

// digest add-sui-generis
var addSuiGenerisCmd = &cobra.Command{
	Use:   "add-sui-generis SECTION RKEY...",
	Short: "Add sui generis picks for a content section",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		section := args[0]
		rkeys := args[1:]

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		return withLock(dir, "content-picks.lock", func() error {
			// Load existing content picks
			contentPicks, err := LoadContentPicks(filepath.Join(dir, "content-picks.json"))
			if err != nil {
				return err
			}

			// Get or create section picks
			picks, ok := contentPicks[section]
			if !ok {
				picks = ContentPicks{SectionID: section, SuiGeneris: []string{}}
			}

			// Add new rkeys (avoid duplicates)
			existing := make(map[string]bool)
			for _, rkey := range picks.SuiGeneris {
				existing[rkey] = true
			}
			for _, rkey := range rkeys {
				if !existing[rkey] {
					picks.SuiGeneris = append(picks.SuiGeneris, rkey)
					existing[rkey] = true
				}
			}

			contentPicks[section] = picks

			// Save
			if err := SaveContentPicks(filepath.Join(dir, "content-picks.json"), contentPicks); err != nil {
				return err
			}

			fmt.Printf("Added %d sui generis picks to '%s'\n", len(rkeys), section)
			return nil
		})
	},
}

// digest move-story
var moveStoryCmd = &cobra.Command{
	Use:   "move-story STORY_ID --to SECTION",
	Short: "Move a story and all its posts to a different section",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storyID := args[0]
		toSection, _ := cmd.Flags().GetString("to")

		if toSection == "" {
			return fmt.Errorf("--to is required")
		}

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		// Use file lock to prevent race conditions
		lockPath := filepath.Join(dir, "categories.lock")
		fileLock := flock.New(lockPath)
		if err := fileLock.Lock(); err != nil {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		defer fileLock.Unlock()

		// Load story groups
		storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
		if err != nil {
			return err
		}

		story, ok := storyGroups[storyID]
		if !ok {
			return fmt.Errorf("story not found: %s", storyID)
		}

		fromSection := story.SectionID
		if fromSection == toSection {
			return fmt.Errorf("story is already in section '%s'", toSection)
		}

		// Load categories to move posts
		cats, err := LoadCategories(filepath.Join(dir, "categories.json"))
		if err != nil {
			return err
		}

		// Move all posts from source to destination section
		for _, rkey := range story.PostRkeys {
			removeFromCategory(rkey, cats)
		}

		// Add to destination section
		destCat := cats[toSection]
		destCat.Visible = append(destCat.Visible, story.PostRkeys...)
		cats[toSection] = destCat

		// Update story
		if story.OriginalSection == "" {
			story.OriginalSection = fromSection
		}
		story.SectionID = toSection
		storyGroups[storyID] = story

		// Save both
		if err := SaveCategories(filepath.Join(dir, "categories.json"), cats); err != nil {
			return err
		}
		if err := SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups); err != nil {
			return err
		}

		headline := story.Headline
		if headline == "" {
			headline = story.DraftHeadline
		}
		if headline == "" {
			headline = "(no headline)"
		}
		fmt.Printf("Moved %s from '%s' to '%s': %s\n", storyID, fromSection, toSection, headline)
		return nil
	},
}

// digest show-front-page
var showFrontPageCmd = &cobra.Command{
	Use:   "show-front-page",
	Short: "Show current front page status",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
		if err != nil {
			return err
		}

		fmt.Println("FRONT PAGE STATUS")
		fmt.Println("=================")

		// Collect front page stories
		var headlines []*StoryGroup
		var featured []*StoryGroup
		var opinions []*StoryGroup

		for id := range storyGroups {
			story := storyGroups[id]
			if story.SectionID != "front-page" {
				continue
			}
			storyCopy := story
			switch story.Role {
			case "headline":
				headlines = append(headlines, &storyCopy)
			case "opinion":
				opinions = append(opinions, &storyCopy)
			default:
				featured = append(featured, &storyCopy)
			}
		}

		// Display headline(s) - show warning if multiple
		if len(headlines) == 0 {
			fmt.Println("\nHeadline: (none)")
		} else if len(headlines) == 1 {
			fmt.Printf("\nHeadline: %s\n", headlines[0].Headline)
			fmt.Printf("  %s (%s)\n", headlines[0].ID, headlines[0].SectionID)
		} else {
			fmt.Printf("\nHeadline: ERROR - %d headlines (only 1 allowed!)\n", len(headlines))
			for _, story := range headlines {
				fmt.Printf("  - %s: %s (%s)\n", story.ID, story.Headline, story.SectionID)
			}
		}

		// Display featured
		fmt.Printf("\nFeatured: %d stories\n", len(featured))
		for _, story := range featured {
			fmt.Printf("  - %s: %s (%s)\n", story.ID, story.Headline, story.SectionID)
		}

		// Display opinions
		fmt.Printf("\nOpinions: %d stories\n", len(opinions))
		for _, story := range opinions {
			fmt.Printf("  - %s: %s (%s)\n", story.ID, story.Headline, story.SectionID)
		}

		return nil
	},
}

// digest show-story
var showStoryCmd = &cobra.Command{
	Use:   "show-story STORY_ID",
	Short: "Show details of a story including all posts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storyID := args[0]

		wd, err := LoadWorkspace(workspaceDir)
		if err != nil {
			dir, _ := GetWorkspaceDir()
			wd, err = LoadWorkspace(dir)
			if err != nil {
				return err
			}
		}

		storyGroups, err := LoadStoryGroups(filepath.Join(wd.Dir, "story-groups.json"))
		if err != nil {
			return err
		}

		story, ok := storyGroups[storyID]
		if !ok {
			return fmt.Errorf("story not found: %s", storyID)
		}

		// Build post index
		postIndex := make(map[string]Post)
		for _, post := range wd.Posts {
			postIndex[post.Rkey] = post
		}

		fmt.Printf("Story: %s - %s\n", story.ID, story.Headline)
		fmt.Printf("Section: %s | Role: %s\n", story.SectionID, story.Role)
		if story.Summary != "" {
			fmt.Printf("Summary: %s\n", story.Summary)
		}
		fmt.Printf("\nPosts (%d):\n", len(story.PostRkeys))

		for _, rkey := range story.PostRkeys {
			isPrimary := rkey == story.PrimaryRkey
			marker := "-"
			suffix := ""
			if isPrimary {
				marker = "*"
				suffix = " [PRIMARY]"
			}

			post, ok := postIndex[rkey]
			if !ok {
				fmt.Printf("  %s %s (post not found)%s\n", marker, rkey, suffix)
				continue
			}

			snippet := post.Text
			if len(snippet) > 80 {
				snippet = snippet[:77] + "..."
			}
			fmt.Printf("  %s %s%s\n", marker, rkey, suffix)
			fmt.Printf("    @%s: \"%s\"\n", post.Author.Handle, snippet)
		}

		return nil
	},
}

// digest create-story-group - for consolidator (no headline/priority required)
var createStoryGroupCmd = &cobra.Command{
	Use:   "create-story-group --section ID --rkeys RKEY...",
	Short: "Create a story group without headline (for consolidation step)",
	RunE: func(cmd *cobra.Command, args []string) error {
		section, _ := cmd.Flags().GetString("section")
		rkeys, _ := cmd.Flags().GetStringSlice("rkeys")
		draftHeadline, _ := cmd.Flags().GetString("draft-headline")
		primary, _ := cmd.Flags().GetString("primary")

		if section == "" {
			return fmt.Errorf("--section is required")
		}
		if len(rkeys) == 0 {
			return fmt.Errorf("--rkeys is required (at least one rkey)")
		}

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		return createStoryGroupInDir(dir, section, rkeys, draftHeadline, primary)
	},
}

// createStoryGroupInDir creates a story group in the given workspace directory
func createStoryGroupInDir(dir, section string, rkeys []string, draftHeadline, primary string) error {
	return withLock(dir, "story-groups.lock", func() error {
		// Load existing story groups
		storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
		if err != nil {
			return err
		}

		// Generate next ID
		nextNum := len(storyGroups) + 1
		id := fmt.Sprintf("sg_%03d", nextNum)
		for {
			if _, exists := storyGroups[id]; !exists {
				break
			}
			nextNum++
			id = fmt.Sprintf("sg_%03d", nextNum)
		}

		// Use first rkey as primary if not specified
		if primary == "" {
			primary = rkeys[0]
		}

		// Create story group (no headline/priority required)
		story := StoryGroup{
			ID:            id,
			DraftHeadline: draftHeadline,
			PostRkeys:     rkeys,
			PrimaryRkey:   primary,
			SectionID:     section,
		}

		storyGroups[id] = story

		// Save
		if err := SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups); err != nil {
			return err
		}

		if draftHeadline != "" {
			fmt.Printf("Created story group %s in '%s': %s (%d posts)\n", id, section, draftHeadline, len(rkeys))
		} else {
			fmt.Printf("Created story group %s in '%s' (%d posts)\n", id, section, len(rkeys))
		}
		return nil
	})
}

// digest show-ungrouped - show posts not in any story group
var showUngroupedCmd = &cobra.Command{
	Use:   "show-ungrouped <section>",
	Short: "Show posts in a section that are not in any story group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		section := args[0]

		wd, err := LoadWorkspace(workspaceDir)
		if err != nil {
			dir, _ := GetWorkspaceDir()
			wd, err = LoadWorkspace(dir)
			if err != nil {
				return err
			}
		}

		storyGroups, err := LoadStoryGroups(filepath.Join(wd.Dir, "story-groups.json"))
		if err != nil {
			return err
		}

		// Get all rkeys in category
		catData, ok := wd.Categories[section]
		if !ok {
			return fmt.Errorf("section not found: %s", section)
		}

		// Build set of rkeys already in story groups for this section
		grouped := make(map[string]bool)
		for _, story := range storyGroups {
			if story.SectionID == section {
				for _, rkey := range story.PostRkeys {
					grouped[rkey] = true
				}
			}
		}

		// Find ungrouped posts
		var ungroupedRkeys []string
		for _, rkey := range catData.Visible {
			if !grouped[rkey] {
				ungroupedRkeys = append(ungroupedRkeys, rkey)
			}
		}

		// Get full posts
		posts := []Post{}
		for _, rkey := range ungroupedRkeys {
			if idx, ok := wd.Index[rkey]; ok && idx < len(wd.Posts) {
				posts = append(posts, wd.Posts[idx])
			}
		}

		displayPosts := FormatForDisplay(posts)

		data, _ := json.MarshalIndent(displayPosts, "", "  ")
		fmt.Println(string(data))

		return nil
	},
}

// digest list-stories - list story groups with filters
var listStoriesCmd = &cobra.Command{
	Use:   "list-stories",
	Short: "List story groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		section, _ := cmd.Flags().GetString("section")
		all, _ := cmd.Flags().GetBool("all")

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
		if err != nil {
			return err
		}

		// Collect and filter stories
		var stories []StoryGroup
		for _, story := range storyGroups {
			if section != "" && story.SectionID != section {
				continue
			}
			stories = append(stories, story)
		}

		if len(stories) == 0 {
			if section != "" {
				fmt.Printf("No stories in section '%s'\n", section)
			} else if all {
				fmt.Println("No stories found")
			}
			return nil
		}

		// Group by section if showing all
		if all || section == "" {
			sectionStories := make(map[string][]StoryGroup)
			for _, story := range stories {
				sectionStories[story.SectionID] = append(sectionStories[story.SectionID], story)
			}

			for sec, secStories := range sectionStories {
				fmt.Printf("\n[%s] %d stories:\n", sec, len(secStories))
				for _, story := range secStories {
					headline := story.Headline
					if headline == "" {
						headline = story.DraftHeadline
					}
					if headline == "" {
						headline = "(no headline)"
					}
					fmt.Printf("  %s (%d posts): %s\n", story.ID, len(story.PostRkeys), headline)
				}
			}
		} else {
			fmt.Printf("[%s] %d stories:\n", section, len(stories))
			for _, story := range stories {
				headline := story.Headline
				if headline == "" {
					headline = story.DraftHeadline
				}
				if headline == "" {
					headline = "(no headline)"
				}
				fmt.Printf("  %s (%d posts): %s\n", story.ID, len(story.PostRkeys), headline)
			}
		}

		return nil
	},
}

// digest update-story - modify an existing story
var updateStoryCmd = &cobra.Command{
	Use:   "update-story STORY_ID --headline TEXT --priority N",
	Short: "Set headline and priority for a story (both required)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storyID := args[0]
		headline, _ := cmd.Flags().GetString("headline")
		role, _ := cmd.Flags().GetString("role")
		priority, _ := cmd.Flags().GetInt("priority")
		opinion, _ := cmd.Flags().GetBool("opinion")
		opinionChanged := cmd.Flags().Changed("opinion")

		// Require both headline and priority
		if headline == "" || priority <= 0 {
			return fmt.Errorf("both --headline and --priority are required")
		}

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		return updateStoryInDir(dir, storyID, headline, priority, role, opinion, opinionChanged)
	},
}

// updateStoryInDir updates a story in the given workspace directory
func updateStoryInDir(dir, storyID, headline string, priority int, role string, opinion, opinionChanged bool) error {
	return withLock(dir, "story-groups.lock", func() error {
		storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
		if err != nil {
			return err
		}

		story, ok := storyGroups[storyID]
		if !ok {
			return fmt.Errorf("story not found: %s", storyID)
		}

		// Check if priority is already used in this section
		for id := range storyGroups {
			existing := storyGroups[id]
			if id != storyID && existing.SectionID == story.SectionID && existing.Priority == priority {
				return fmt.Errorf("priority %d already used in section '%s' by story '%s'", priority, story.SectionID, id)
			}
		}

		// Update fields
		story.Headline = headline
		story.Priority = priority

		if role != "" {
			story.Role = role
		}
		if opinionChanged {
			story.IsOpinion = opinion
			if opinion {
				story.Role = "opinion"
			}
		}

		storyGroups[storyID] = story

		if err := SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups); err != nil {
			return err
		}

		fmt.Printf("Updated %s: headline=%q priority=%d\n", storyID, headline, priority)
		return nil
	})
}

// digest show-unprocessed - show stories without headline or priority
var showUnprocessedCmd = &cobra.Command{
	Use:   "show-unprocessed [section-id]",
	Short: "List stories that need headline and priority set",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filterSection := ""
		if len(args) == 1 {
			filterSection = args[0]
		}

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
		if err != nil {
			return err
		}

		// Group unprocessed stories by section
		unprocessed := make(map[string][]string)
		for id, story := range storyGroups {
			if filterSection != "" && story.SectionID != filterSection {
				continue
			}
			if story.Headline == "" || story.Priority == 0 {
				unprocessed[story.SectionID] = append(unprocessed[story.SectionID], id)
			}
		}

		if len(unprocessed) == 0 {
			fmt.Println("All stories have headlines and priorities set!")
			return nil
		}

		// Sort sections for consistent output
		var sections []string
		for section := range unprocessed {
			sections = append(sections, section)
		}
		sort.Strings(sections)

		total := 0
		for _, section := range sections {
			ids := unprocessed[section]
			sort.Strings(ids)
			fmt.Printf("[%s] %d unprocessed:\n", section, len(ids))
			for _, id := range ids {
				story := storyGroups[id]
				missing := []string{}
				if story.Headline == "" {
					missing = append(missing, "headline")
				}
				if story.Priority == 0 {
					missing = append(missing, "priority")
				}
				fmt.Printf("  %s (missing: %s)\n", id, joinStrings(missing, ", "))
			}
			total += len(ids)
			fmt.Println()
		}
		fmt.Printf("Total unprocessed: %d stories\n", total)
		return nil
	},
}

// digest auto-group-remaining - wrap ungrouped posts into single-post stories
var autoGroupRemainingCmd = &cobra.Command{
	Use:   "auto-group-remaining",
	Short: "Create single-post story groups for all ungrouped posts",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		cats, err := LoadCategories(filepath.Join(dir, "categories.json"))
		if err != nil {
			return err
		}

		return withLock(dir, "story-groups.lock", func() error {
			storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
			if err != nil {
				return err
			}

			// Build set of all rkeys in story groups by section
			groupedBySection := make(map[string]map[string]bool)
			for _, story := range storyGroups {
				if groupedBySection[story.SectionID] == nil {
					groupedBySection[story.SectionID] = make(map[string]bool)
				}
				for _, rkey := range story.PostRkeys {
					groupedBySection[story.SectionID][rkey] = true
				}
			}

			// Create story groups for ungrouped posts
			created := 0
			for section, catData := range cats {
				if catData.IsHidden {
					continue
				}
				grouped := groupedBySection[section]
				if grouped == nil {
					grouped = make(map[string]bool)
				}

				for _, rkey := range catData.Visible {
					if grouped[rkey] {
						continue
					}

					// Generate next ID
					nextNum := len(storyGroups) + 1
					id := fmt.Sprintf("sg_%03d", nextNum)
					for {
						if _, exists := storyGroups[id]; !exists {
							break
						}
						nextNum++
						id = fmt.Sprintf("sg_%03d", nextNum)
					}

					story := StoryGroup{
						ID:          id,
						PostRkeys:   []string{rkey},
						PrimaryRkey: rkey,
						SectionID:   section,
					}
					storyGroups[id] = story
					created++
				}
			}

			if created == 0 {
				fmt.Println("All posts are already in story groups")
				return nil
			}

			if err := SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups); err != nil {
				return err
			}

			fmt.Printf("Created %d single-post story groups\n", created)
			return nil
		})
	},
}

// digest add-to-story - add posts to an existing story
var addToStoryCmd = &cobra.Command{
	Use:   "add-to-story STORY_ID RKEY...",
	Short: "Add posts to an existing story group",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		storyID := args[0]
		newRkeys := args[1:]

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		return withLock(dir, "story-groups.lock", func() error {
			storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
			if err != nil {
				return err
			}

			story, ok := storyGroups[storyID]
			if !ok {
				return fmt.Errorf("story not found: %s", storyID)
			}

			// Add new rkeys (avoid duplicates)
			existing := make(map[string]bool)
			for _, rkey := range story.PostRkeys {
				existing[rkey] = true
			}
			added := 0
			for _, rkey := range newRkeys {
				if !existing[rkey] {
					story.PostRkeys = append(story.PostRkeys, rkey)
					existing[rkey] = true
					added++
				}
			}

			if added == 0 {
				fmt.Printf("All rkeys already in story %s\n", storyID)
				return nil
			}

			storyGroups[storyID] = story

			if err := SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups); err != nil {
				return err
			}

			fmt.Printf("Added %d posts to %s (now %d total)\n", added, storyID, len(story.PostRkeys))
			return nil
		})
	},
}

// BatchProgress tracks completion of parallel workflow stages
type BatchProgress struct {
	Categorization []CatBatch `json:"categorization,omitempty"`
	Consolidation  []string   `json:"consolidation,omitempty"`
	Headlines      []string   `json:"headlines,omitempty"`
}

type CatBatch struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

func loadBatchProgress(dir string) (*BatchProgress, error) {
	path := filepath.Join(dir, "batches.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &BatchProgress{}, nil
		}
		return nil, err
	}
	var bp BatchProgress
	if err := json.Unmarshal(data, &bp); err != nil {
		return nil, err
	}
	return &bp, nil
}

func saveBatchProgress(dir string, bp *BatchProgress) error {
	path := filepath.Join(dir, "batches.json")
	data, err := json.MarshalIndent(bp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// digest mark-batch-done
var markBatchDoneCmd = &cobra.Command{
	Use:   "mark-batch-done",
	Short: "Mark a workflow batch as complete",
	Long: `Mark a workflow stage batch as complete.

For categorization:
  ./bin/digest mark-batch-done --stage categorization --offset 0 --limit 100

For consolidation:
  ./bin/digest mark-batch-done --stage consolidation --section music

For headlines:
  ./bin/digest mark-batch-done --stage headlines --section music`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stage, _ := cmd.Flags().GetString("stage")
		offset, _ := cmd.Flags().GetInt("offset")
		limit, _ := cmd.Flags().GetInt("limit")
		section, _ := cmd.Flags().GetString("section")

		if stage == "" {
			return fmt.Errorf("--stage is required (categorization, consolidation, headlines)")
		}

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		return markBatchDoneInDir(dir, stage, offset, limit, section)
	},
}

// markBatchDoneInDir marks a batch as complete in the given workspace directory
func markBatchDoneInDir(dir, stage string, offset, limit int, section string) error {
	return withLock(dir, "batches.lock", func() error {
		bp, err := loadBatchProgress(dir)
		if err != nil {
			return err
		}

		switch stage {
		case "categorization":
			if limit == 0 {
				return fmt.Errorf("--limit is required for categorization stage")
			}
			// Check if already recorded
			for _, b := range bp.Categorization {
				if b.Offset == offset && b.Limit == limit {
					fmt.Printf("Batch %d-%d already marked complete\n", offset, offset+limit)
					return nil
				}
			}
			bp.Categorization = append(bp.Categorization, CatBatch{Offset: offset, Limit: limit})
			fmt.Printf("Marked categorization batch %d-%d complete\n", offset, offset+limit)

		case "consolidation":
			if section == "" {
				return fmt.Errorf("--section is required for consolidation stage")
			}
			for _, s := range bp.Consolidation {
				if s == section {
					fmt.Printf("Consolidation for %s already marked complete\n", section)
					return nil
				}
			}
			bp.Consolidation = append(bp.Consolidation, section)
			fmt.Printf("Marked consolidation for %s complete\n", section)

		case "headlines":
			if section == "" {
				return fmt.Errorf("--section is required for headlines stage")
			}
			for _, s := range bp.Headlines {
				if s == section {
					fmt.Printf("Headlines for %s already marked complete\n", section)
					return nil
				}
			}
			bp.Headlines = append(bp.Headlines, section)
			fmt.Printf("Marked headlines for %s complete\n", section)

		default:
			return fmt.Errorf("unknown stage: %s (use categorization, consolidation, or headlines)", stage)
		}

		return saveBatchProgress(dir, bp)
	})
}

func init() {
	// init flags
	initCmd.Flags().String("since", "", "Start time for fetching (default: 24h ago)")

	// fetch flags
	fetchCmd.Flags().Int("limit", 0, "Max posts to fetch (0 = unlimited)")

	// read-posts flags
	readPostsCmd.Flags().Int("offset", 0, "Skip first N posts")
	readPostsCmd.Flags().Int("limit", 20, "Show M posts")
	readPostsCmd.Flags().Bool("images", false, "Include base64-encoded images for agent viewing")

	// categorize flags
	categorizeCmd.Flags().Bool("move", false, "Move posts from existing category (for front-page selection)")

	// list-categories flags
	listCategoriesCmd.Flags().Bool("with-counts", true, "Show post counts")

	// compile flags
	compileCmd.Flags().String("output", "", "Output file (default: workspace/digest.md)")

	// move-story flags
	moveStoryCmd.Flags().String("to", "", "Destination section ID (required)")

	// create-story-group flags
	createStoryGroupCmd.Flags().String("section", "", "Section ID (required)")
	createStoryGroupCmd.Flags().StringSlice("rkeys", nil, "Post rkeys for this story (required)")
	createStoryGroupCmd.Flags().String("draft-headline", "", "Optional draft headline")
	createStoryGroupCmd.Flags().String("primary", "", "Primary rkey (default: first rkey)")

	// list-stories flags
	listStoriesCmd.Flags().String("section", "", "Filter by section")
	listStoriesCmd.Flags().Bool("all", false, "Show all stories")

	// update-story flags
	updateStoryCmd.Flags().String("headline", "", "Set headline")
	updateStoryCmd.Flags().String("role", "", "Set role (headline, featured, opinion)")
	updateStoryCmd.Flags().Int("priority", 0, "Set priority (1 = highest)")
	updateStoryCmd.Flags().Bool("opinion", false, "Mark as opinion piece")

	// mark-batch-done flags
	markBatchDoneCmd.Flags().String("stage", "", "Stage name (categorization, consolidation, headlines)")
	markBatchDoneCmd.Flags().Int("offset", 0, "Batch offset (for categorization)")
	markBatchDoneCmd.Flags().Int("limit", 0, "Batch limit (for categorization)")
	markBatchDoneCmd.Flags().String("section", "", "Section ID (for consolidation/headlines)")
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}
