package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	version      = "0.1.0"
	workspaceDir string // Global flag for workspace directory

	// Status command flags
	statusWaitFor  string
	statusTimeout  int
	statusInterval int
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already printed the error, just exit
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "digest",
	Short: "Bluesky digest tool",
	Long: `A CLI tool for creating daily digests from Bluesky timeline.
Fetches posts, categorizes them, and generates narrative summaries.`,
	Version: version,
	// SilenceUsage prevents showing usage on every error
	SilenceUsage: false,
	// SilenceErrors lets us handle error printing
	SilenceErrors: false,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&workspaceDir, "dir", "", "Workspace directory (default: auto-detect current digest-*)")

	// Add all subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(readPostsCmd)
	rootCmd.AddCommand(categorizeCmd)
	rootCmd.AddCommand(listCategoriesCmd)
	rootCmd.AddCommand(showCategoryCmd)
	rootCmd.AddCommand(compileCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(uncategorizedCmd)
	rootCmd.AddCommand(addSuiGenerisCmd)
	rootCmd.AddCommand(moveStoryCmd)
	rootCmd.AddCommand(showFrontPageCmd)
	rootCmd.AddCommand(showStoryCmd)
	rootCmd.AddCommand(createStoryGroupCmd)
	rootCmd.AddCommand(showUngroupedCmd)
	rootCmd.AddCommand(listStoriesCmd)
	rootCmd.AddCommand(updateStoryCmd)
	rootCmd.AddCommand(showUnprocessedCmd)
	rootCmd.AddCommand(autoGroupRemainingCmd)
	rootCmd.AddCommand(addToStoryCmd)
	rootCmd.AddCommand(markBatchDoneCmd)

	// Status command flags
	statusCmd.Flags().StringVar(&statusWaitFor, "wait-for", "", "Block until stage completes (categorization|consolidation|headlines)")
	statusCmd.Flags().IntVar(&statusTimeout, "timeout", 600, "Timeout in seconds when using --wait-for")
	statusCmd.Flags().IntVar(&statusInterval, "interval", 5, "Poll interval in seconds when using --wait-for")
}
