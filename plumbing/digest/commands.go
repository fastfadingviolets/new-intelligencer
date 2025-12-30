package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
)

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
		SaveSummaries(filepath.Join(dirName, "summaries.json"), Summaries{})

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

		wd, err := LoadWorkspace(workspaceDir)
		if err != nil {
			dir, _ := GetWorkspaceDir()
			wd, err = LoadWorkspace(dir)
			if err != nil {
				return err
			}
		}

		// Categorize
		if err := CategorizePosts(wd.Categories, wd.Index, category, rkeys); err != nil {
			return err
		}

		// Save
		if err := SaveCategories(filepath.Join(wd.Dir, "categories.json"), wd.Categories); err != nil {
			return err
		}

		fmt.Printf("Categorized %d posts into '%s'\n", len(rkeys), category)
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

// digest write-summary
var writeSummaryCmd = &cobra.Command{
	Use:   "write-summary <category> <text>",
	Short: "Write summary for a category",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		category, summary := args[0], args[1]

		dir, err := GetWorkspaceDir()
		if err != nil {
			return err
		}

		sums, err := LoadSummaries(filepath.Join(dir, "summaries.json"))
		if err != nil {
			return err
		}

		sums[category] = summary

		if err := SaveSummaries(filepath.Join(dir, "summaries.json"), sums); err != nil {
			return err
		}

		fmt.Printf("Updated summary for '%s'\n", category)
		return nil
	},
}

// digest compile
var compileCmd = &cobra.Command{
	Use:   "compile",
	Short: "Generate final markdown digest",
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		format, _ := cmd.Flags().GetString("format")

		wd, err := LoadWorkspace(workspaceDir)
		if err != nil {
			dir, _ := GetWorkspaceDir()
			wd, err = LoadWorkspace(dir)
			if err != nil {
				return err
			}
		}

		config := Config{CreatedAt: time.Now()} // TODO: Load from file

		var markdown string
		if format == "newspaper" {
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
			sectionAssignments, err := LoadSectionAssignments(filepath.Join(wd.Dir, "section-assignments.json"))
			if err != nil {
				return err
			}
			contentPicks, err := LoadContentPicks(filepath.Join(wd.Dir, "content-picks.json"))
			if err != nil {
				return err
			}

			markdown, err = CompileNewspaper(wd.Posts, wd.Categories, wd.Summaries, storyGroups, newspaperConfig, sectionAssignments, contentPicks, config)
			if err != nil {
				return err
			}
		} else {
			// Classic format
			markdown, err = CompileDigest(wd.Posts, wd.Categories, wd.Summaries, config)
			if err != nil {
				return err
			}
		}

		if output == "" {
			output = filepath.Join(wd.Dir, "digest.md")
		}

		if err := os.WriteFile(output, []byte(markdown), 0644); err != nil {
			return err
		}

		fmt.Printf("Compiled digest to %s\n", output)
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
			hasSum := "✓"
			if _, ok := wd.Summaries[cat]; !ok {
				hasSum = " "
			}

			catData := wd.Categories[cat]
			hiddenPostCount := len(catData.Hidden)

			if hiddenPostCount > 0 {
				fmt.Printf("  [%s] %s: %d visible, %d hidden\n", hasSum, cat, count, hiddenPostCount)
			} else {
				fmt.Printf("  [%s] %s: %d posts\n", hasSum, cat, count)
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

func init() {
	// init flags
	initCmd.Flags().String("since", "", "Start time for fetching (default: 24h ago)")

	// fetch flags
	fetchCmd.Flags().Int("limit", 0, "Max posts to fetch (0 = unlimited)")

	// read-posts flags
	readPostsCmd.Flags().Int("offset", 0, "Skip first N posts")
	readPostsCmd.Flags().Int("limit", 20, "Show M posts")

	// list-categories flags
	listCategoriesCmd.Flags().Bool("with-counts", true, "Show post counts")

	// compile flags
	compileCmd.Flags().String("output", "", "Output file (default: workspace/digest.md)")
	compileCmd.Flags().String("format", "classic", "Output format: classic or newspaper")

	// hide-posts flags
	hidePostsCmd.Flags().String("reason", "", "Reason for hiding posts")
}
