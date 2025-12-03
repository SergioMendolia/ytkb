package cmd

import (
	"github.com/spf13/cobra"
	"ytkb/internal/config"
)

var cfg *config.Config

func Execute(c *config.Config) error {
	cfg = c

	rootCmd := &cobra.Command{
		Use:   "youtrack_writer",
		Short: "Sync YouTrack knowledge base articles",
		Long:  "A CLI tool to download, diff, and push YouTrack knowledge base articles",
	}

	rootCmd.AddCommand(downloadCmd())
	rootCmd.AddCommand(diffCmd())
	rootCmd.AddCommand(pushCmd())

	return rootCmd.Execute()
}
