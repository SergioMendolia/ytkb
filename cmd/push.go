package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"ytkb/internal/api"
	"ytkb/internal/filesystem"
	"ytkb/internal/markdown"

	"github.com/spf13/cobra"
)

func pushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push [page]",
		Short: "Push changes to server",
		Long:  "Push changes to server. If page is specified, push that page. Otherwise, push all changes.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runPush,
	}
	return cmd
}

func runPush(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return pushSinglePage(args[0])
	}
	return pushAllChanges()
}

func pushSinglePage(filePath string) error {
	fmt.Printf("Pushing %s...\n", filePath)

	content, err := filesystem.ReadMarkdownFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	md, err := markdown.ParseMarkdown(content)
	if err != nil {
		return fmt.Errorf("failed to parse markdown: %w", err)
	}

	client := api.NewClient(cfg)

	// Check if article exists
	if md.Frontmatter.ID == "" {
		return fmt.Errorf("cannot push article without ID: %s. Articles must be created manually in YouTrack first", md.Frontmatter.Title)
	}

	// Update existing article
	_, err = client.UpdateArticle(md.Frontmatter.ID, md.Frontmatter.Title, md.Content)
	if err != nil {
		return fmt.Errorf("failed to update article: %w", err)
	}
	fmt.Printf("Updated: %s\n", md.Frontmatter.Title)

	return nil
}

func pushAllChanges() error {
	// Get diff
	localFiles, err := filesystem.FindMarkdownFiles(".")
	if err != nil {
		return fmt.Errorf("failed to find local files: %w", err)
	}

	client := api.NewClient(cfg)
	serverArticles, err := client.ListArticles()
	if err != nil {
		return fmt.Errorf("failed to list server articles: %w", err)
	}

	// Build maps
	localByID := make(map[string]*markdown.MarkdownFile)
	localPaths := make(map[string]string)
	serverByID := make(map[string]*api.Article)

	for _, filePath := range localFiles {
		content, err := filesystem.ReadMarkdownFile(filePath)
		if err != nil {
			continue
		}

		md, err := markdown.ParseMarkdown(content)
		if err != nil {
			continue
		}

		if md.Frontmatter.ID != "" {
			localByID[md.Frontmatter.ID] = md
			localPaths[md.Frontmatter.ID] = filePath
		}
	}

	for i := range serverArticles {
		serverByID[serverArticles[i].ID] = &serverArticles[i]
	}

	// Collect pages to push (modified)
	var pagesToPush []struct {
		id       string
		title    string
		filePath string
	}

	for id, localMD := range localByID {
		if serverArticle, ok := serverByID[id]; ok {
			localContent := strings.TrimSpace(localMD.Content)
			serverContent := strings.TrimSpace(serverArticle.Content)

			if localContent != serverContent {
				filePath := localPaths[id]
				pagesToPush = append(pagesToPush, struct {
					id       string
					title    string
					filePath string
				}{id: id, title: localMD.Frontmatter.Title, filePath: filePath})
			}
		}
	}

	// Skip new pages (cannot create articles)
	var newPages []string
	for _, filePath := range localFiles {
		content, err := filesystem.ReadMarkdownFile(filePath)
		if err != nil {
			continue
		}

		md, err := markdown.ParseMarkdown(content)
		if err != nil {
			continue
		}

		// New page (no ID) or not on server
		_, existsOnServer := serverByID[md.Frontmatter.ID]
		isNew := md.Frontmatter.ID == "" || !existsOnServer
		if isNew {
			newPages = append(newPages, filePath)
		}
	}

	// Show what will be pushed
	if len(pagesToPush) == 0 && len(newPages) == 0 {
		fmt.Println("No changes to push.")
		return nil
	}

	fmt.Println("\nPages to be pushed:")
	if len(pagesToPush) > 0 {
		for i, page := range pagesToPush {
			fmt.Printf("  %d. %s (%s)\n", i+1, page.title, page.filePath)
		}
	}

	if len(newPages) > 0 {
		fmt.Printf("\n⚠️  Skipped %d new articles (creation not supported):\n", len(newPages))
		for _, path := range newPages {
			fmt.Printf("   %s\n", path)
		}
		fmt.Println("   Create these articles manually in YouTrack first, then download to get their IDs.")
	}

	// Ask for confirmation
	fmt.Print("\nProceed with push? (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Push cancelled.")
		return nil
	}

	// Process modified pages (update)
	fmt.Println("\nPushing changes...")
	for _, page := range pagesToPush {
		localMD := localByID[page.id]
		_, err := client.UpdateArticle(page.id, localMD.Frontmatter.Title, localMD.Content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update %s: %v\n", page.title, err)
			continue
		}
		fmt.Printf("Updated: %s\n", page.title)
	}

	// Process deleted pages (warn only)
	for id, article := range serverByID {
		if _, exists := localByID[id]; !exists {
			fmt.Printf("⚠️  Page deleted locally: %s\n", article.Title)
			fmt.Printf("   Delete manually at: %s\n", article.URL)
		}
	}

	fmt.Println("\nPush complete.")
	return nil
}
