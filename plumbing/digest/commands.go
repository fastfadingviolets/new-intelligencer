package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
)

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

		posts, err := LoadPosts(filepath.Join(dir, "posts.json"))
		if err != nil {
			return err
		}

		// Apply offset and limit
		end := len(posts)
		if offset >= end {
			offset = end
		}
		if limit > 0 && offset+limit < end {
			end = offset + limit
		}

		posts = posts[offset:end]
		displayPosts := FormatForDisplay(posts)

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

		// Use file lock to prevent race conditions with parallel agents
		lockPath := filepath.Join(dir, "categories.lock")
		fileLock := flock.New(lockPath)
		if err := fileLock.Lock(); err != nil {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		defer fileLock.Unlock()

		// Load workspace AFTER acquiring lock (data may have changed)
		wd, err := LoadWorkspace(dir)
		if err != nil {
			return err
		}

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
	},
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

// digest merge-categories
var mergeCategoriesCmd = &cobra.Command{
	Use:   "merge-categories <from> <to>",
	Short: "Merge two categories",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		from, to := args[0], args[1]

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		cats, err := LoadCategories(filepath.Join(dir, "categories.json"))
		if err != nil {
			return err
		}

		if err := MergeCategories(cats, from, to); err != nil {
			return err
		}

		if err := SaveCategories(filepath.Join(dir, "categories.json"), cats); err != nil {
			return err
		}

		fmt.Printf("Merged '%s' into '%s'\n", from, to)
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

		// Load newspaper config from project root
		newspaperConfig, err := LoadNewspaperConfig("newspaper.json")
		if err != nil {
			return fmt.Errorf("loading newspaper.json from project root: %w", err)
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

// digest hide-category
var hideCategoryCmd = &cobra.Command{
	Use:   "hide-category <category>",
	Short: "Hide a category from the digest",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		category := args[0]

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		// Load categories
		cats, err := LoadCategories(filepath.Join(dir, "categories.json"))
		if err != nil {
			return err
		}

		// Hide category
		postCount, err := HideCategory(cats, category)
		if err != nil {
			return err
		}

		// Save
		if err := SaveCategories(filepath.Join(dir, "categories.json"), cats); err != nil {
			return err
		}

		fmt.Printf("Hidden category '%s' (%d posts)\n", category, postCount)
		return nil
	},
}

// digest unhide-category
var unhideCategoryCmd = &cobra.Command{
	Use:   "unhide-category <category>",
	Short: "Unhide a category to show it in the digest",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		category := args[0]

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		// Load categories
		cats, err := LoadCategories(filepath.Join(dir, "categories.json"))
		if err != nil {
			return err
		}

		// Unhide category
		if err := UnhideCategory(cats, category); err != nil {
			return err
		}

		// Save
		if err := SaveCategories(filepath.Join(dir, "categories.json"), cats); err != nil {
			return err
		}

		fmt.Printf("Unhidden category '%s'\n", category)
		return nil
	},
}

// digest hide-posts
var hidePostsCmd = &cobra.Command{
	Use:   "hide-posts <category> <rkey>... [--reason TEXT]",
	Short: "Hide posts within a category",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		category := args[0]
		rkeys := args[1:]
		reason, _ := cmd.Flags().GetString("reason")

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		// Load categories
		cats, err := LoadCategories(filepath.Join(dir, "categories.json"))
		if err != nil {
			return err
		}

		// Hide posts
		if err := HidePosts(cats, category, rkeys, reason); err != nil {
			return err
		}

		// Save
		if err := SaveCategories(filepath.Join(dir, "categories.json"), cats); err != nil {
			return err
		}

		if reason != "" {
			fmt.Printf("Hid %d posts in '%s' (reason: %s)\n", len(rkeys), category, reason)
		} else {
			fmt.Printf("Hid %d posts in '%s'\n", len(rkeys), category)
		}
		return nil
	},
}

// digest delete-category
var deleteCategoryCmd = &cobra.Command{
	Use:   "delete-category <category>",
	Short: "Delete a category entirely",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		category := args[0]

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		cats, err := LoadCategories(filepath.Join(dir, "categories.json"))
		if err != nil {
			return err
		}

		catData, ok := cats[category]
		if !ok {
			return fmt.Errorf("category not found: %s", category)
		}

		postCount := len(catData.Visible) + len(catData.Hidden)
		delete(cats, category)

		if err := SaveCategories(filepath.Join(dir, "categories.json"), cats); err != nil {
			return err
		}

		fmt.Printf("Deleted category '%s' (%d posts now uncategorized)\n", category, postCount)
		return nil
	},
}

// digest create-story
var createStoryCmd = &cobra.Command{
	Use:   "create-story --section ID --headline TEXT --rkeys RKEY...",
	Short: "Create a story group for a news section",
	RunE: func(cmd *cobra.Command, args []string) error {
		section, _ := cmd.Flags().GetString("section")
		headline, _ := cmd.Flags().GetString("headline")
		rkeys, _ := cmd.Flags().GetStringSlice("rkeys")
		role, _ := cmd.Flags().GetString("role")
		primary, _ := cmd.Flags().GetString("primary")
		summary, _ := cmd.Flags().GetString("summary")
		articleURL, _ := cmd.Flags().GetString("article-url")
		opinion, _ := cmd.Flags().GetBool("opinion")
		frontPage, _ := cmd.Flags().GetBool("front-page")

		if section == "" {
			return fmt.Errorf("--section is required")
		}
		if headline == "" {
			return fmt.Errorf("--headline is required")
		}
		if len(rkeys) == 0 {
			return fmt.Errorf("--rkeys is required (at least one rkey)")
		}

		// Opinion pieces should have role "opinion"
		if opinion && role != "opinion" {
			role = "opinion"
		}

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		// Load existing story groups
		storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
		if err != nil {
			return err
		}

		// Check if section already has a headline (only one allowed per section)
		if role == "headline" {
			for id := range storyGroups {
				existing := storyGroups[id]
				if existing.SectionID == section && existing.Role == "headline" {
					return fmt.Errorf("section '%s' already has a headline: %s (%s)", section, id, existing.Headline)
				}
			}
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

		// Create story group
		story := StoryGroup{
			ID:          id,
			Headline:    headline,
			Summary:     summary,
			ArticleURL:  articleURL,
			PostRkeys:   rkeys,
			PrimaryRkey: primary,
			IsOpinion:   opinion,
			SectionID:   section,
			Role:        role,
			IsFrontPage: frontPage,
		}

		storyGroups[id] = story

		// Save
		if err := SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups); err != nil {
			return err
		}

		fmt.Printf("Created story %s: %s\n", id, headline)
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
	},
}

// digest set-front-page
var setFrontPageCmd = &cobra.Command{
	Use:   "set-front-page STORY_ID",
	Short: "Mark a story for front page display",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storyID := args[0]

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		// Load story groups
		storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
		if err != nil {
			return err
		}

		story, ok := storyGroups[storyID]
		if !ok {
			return fmt.Errorf("story not found: %s", storyID)
		}

		story.IsFrontPage = true
		storyGroups[storyID] = story

		// Save
		if err := SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups); err != nil {
			return err
		}

		fmt.Printf("Marked %s for front page: %s\n", storyID, story.Headline)
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

// digest set-primary
var setPrimaryCmd = &cobra.Command{
	Use:   "set-primary STORY_ID RKEY",
	Short: "Set the primary post for a story",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		storyID := args[0]
		newPrimary := args[1]

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
		if err != nil {
			return err
		}

		story, ok := storyGroups[storyID]
		if !ok {
			return fmt.Errorf("story not found: %s", storyID)
		}

		// Verify rkey is in story's posts
		found := false
		for _, rkey := range story.PostRkeys {
			if rkey == newPrimary {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("rkey %s is not in story %s", newPrimary, storyID)
		}

		story.PrimaryRkey = newPrimary
		storyGroups[storyID] = story

		if err := SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups); err != nil {
			return err
		}

		fmt.Printf("Set primary for %s to %s\n", storyID, newPrimary)
		return nil
	},
}

func init() {
	// init flags
	initCmd.Flags().String("since", "", "Start time for fetching (default: 24h ago)")

	// fetch flags
	fetchCmd.Flags().Int("limit", 0, "Max posts to fetch (0 = unlimited)")

	// read-posts flags
	readPostsCmd.Flags().Int("offset", 0, "Skip first N posts")
	readPostsCmd.Flags().Int("limit", 20, "Show M posts")

	// categorize flags
	categorizeCmd.Flags().Bool("move", false, "Move posts from existing category (for front-page selection)")

	// list-categories flags
	listCategoriesCmd.Flags().Bool("with-counts", true, "Show post counts")

	// compile flags
	compileCmd.Flags().String("output", "", "Output file (default: workspace/digest.md)")

	// hide-posts flags
	hidePostsCmd.Flags().String("reason", "", "Reason for hiding posts")

	// create-story flags
	createStoryCmd.Flags().String("section", "", "Section ID (required)")
	createStoryCmd.Flags().String("headline", "", "Story headline (required)")
	createStoryCmd.Flags().StringSlice("rkeys", nil, "Post rkeys for this story (required)")
	createStoryCmd.Flags().String("role", "featured", "Story role: headline, featured, or opinion")
	createStoryCmd.Flags().String("primary", "", "Primary rkey (default: first rkey)")
	createStoryCmd.Flags().String("summary", "", "Optional summary text")
	createStoryCmd.Flags().String("article-url", "", "Optional article URL")
	createStoryCmd.Flags().Bool("opinion", false, "Mark as opinion piece")
	createStoryCmd.Flags().Bool("front-page", false, "Mark for front page")
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
