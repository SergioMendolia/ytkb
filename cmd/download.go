package cmd

import (
	"fmt"
	"path/filepath"
	"sort"

	"ytkb/internal/api"
	"ytkb/internal/filesystem"
	"ytkb/internal/markdown"

	"github.com/spf13/cobra"
)

func downloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "download",
		Short: "Download all pages from knowledge base",
		RunE:  runDownload,
	}
}

func runDownload(cmd *cobra.Command, args []string) error {
	fmt.Println("Downloading knowledge base articles...")

	client := api.NewClient(cfg)
	articles, err := client.ListArticles()
	if err != nil {
		return fmt.Errorf("failed to list articles: %w", err)
	}

	if len(articles) == 0 {
		fmt.Println("No articles found.")
		return nil
	}

	// Build map of articles by ID for quick lookup
	articlesByID := make(map[string]*api.Article)
	for i := range articles {
		articlesByID[articles[i].ID] = &articles[i]
	}

	// Find all root articles (no parent)
	var rootArticles []*api.Article
	for i := range articles {
		if articles[i].ParentID == nil || *articles[i].ParentID == "" {
			rootArticles = append(rootArticles, &articles[i])
		}
	}

	// Sort root articles by order
	sort.Slice(rootArticles, func(i, j int) bool {
		return rootArticles[i].Order < rootArticles[j].Order
	})

	fmt.Printf("Found %d root articles\n", len(rootArticles))

	// Download each root article and its children recursively
	basePath := "."
	for _, rootArticle := range rootArticles {
		if err := downloadArticleRecursive(rootArticle, basePath, articlesByID); err != nil {
			return err
		}
	}

	fmt.Printf("Downloaded %d articles.\n", len(articles))
	return nil
}

// downloadArticleRecursive downloads an article and recursively downloads its children
func downloadArticleRecursive(article *api.Article, basePath string, articlesByID map[string]*api.Article) error {
	sanitizedTitle := filesystem.SanitizeFilename(article.Title)
	filePath := filepath.Join(basePath, sanitizedTitle+".md")

	fmt.Printf("Downloading: %s -> %s\n", article.Title, filePath)

	// Save article
	frontmatter := markdown.Frontmatter{
		ID:    article.ID,
		Title: article.Title,
		URL:   article.URL,
	}

	content, err := markdown.WriteMarkdown(frontmatter, article.Content)
	if err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	if err := filesystem.WriteMarkdownFile(filePath, content); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	// Find all children of this article
	var children []*api.Article
	for i := range articlesByID {
		child := articlesByID[i]
		if child.ParentID != nil && *child.ParentID == article.ID {
			children = append(children, child)
		}
	}

	// Sort children by order
	sort.Slice(children, func(i, j int) bool {
		return children[i].Order < children[j].Order
	})

	// If there are children, create a folder and download them recursively
	if len(children) > 0 {
		childDir := filepath.Join(basePath, sanitizedTitle)
		fmt.Printf("Creating folder for %s: %s (with %d children)\n", article.Title, childDir, len(children))

		for _, child := range children {
			if err := downloadArticleRecursive(child, childDir, articlesByID); err != nil {
				return err
			}
		}
	}

	return nil
}
