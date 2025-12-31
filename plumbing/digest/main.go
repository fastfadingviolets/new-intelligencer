package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	workspaceDir string // Global flag for workspace directory
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
	rootCmd.AddCommand(mergeCategoriesCmd)
	rootCmd.AddCommand(hideCategoryCmd)
	rootCmd.AddCommand(unhideCategoryCmd)
	rootCmd.AddCommand(hidePostsCmd)
	rootCmd.AddCommand(compileCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(uncategorizedCmd)
	rootCmd.AddCommand(deleteCategoryCmd)
	rootCmd.AddCommand(createStoryCmd)
	rootCmd.AddCommand(addSuiGenerisCmd)
	rootCmd.AddCommand(setFrontPageCmd)
	rootCmd.AddCommand(showFrontPageCmd)
	rootCmd.AddCommand(showStoryCmd)
	rootCmd.AddCommand(setPrimaryCmd)
}
