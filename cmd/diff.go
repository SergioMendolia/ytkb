package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"ytkb/internal/api"
	"ytkb/internal/filesystem"
	"ytkb/internal/markdown"

	"github.com/spf13/cobra"
)

type ArticleStatus int

const (
	StatusUnchanged ArticleStatus = iota
	StatusModified
	StatusNewLocal
	StatusDeleted
)

type ArticleNode struct {
	ID       string
	Title    string
	Status   ArticleStatus
	Children []*ArticleNode
	Path     string
}

func diffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show differences between local and server",
		RunE:  runDiff,
	}
}

func runDiff(cmd *cobra.Command, args []string) error {
	fmt.Println("Comparing local files with server...")

	// Get local files
	localFiles, err := filesystem.FindMarkdownFiles(".")
	if err != nil {
		return fmt.Errorf("failed to find local files: %w", err)
	}

	// Get server articles
	client := api.NewClient(cfg)
	serverArticles, err := client.ListArticles()
	if err != nil {
		return fmt.Errorf("failed to list server articles: %w", err)
	}

	// Build maps
	localByID := make(map[string]*markdown.MarkdownFile)
	localPaths := make(map[string]string)
	localByPath := make(map[string]*markdown.MarkdownFile) // For new files without ID
	serverByID := make(map[string]*api.Article)

	// Load local files
	for _, filePath := range localFiles {
		content, err := filesystem.ReadMarkdownFile(filePath)
		if err != nil {
			continue
		}

		md, err := markdown.ParseMarkdown(content)
		if err != nil {
			continue
		}

		// Index by ID if exists
		if md.Frontmatter.ID != "" {
			localByID[md.Frontmatter.ID] = md
			localPaths[md.Frontmatter.ID] = filePath
		} else {
			// New file without ID - index by path
			localByPath[filePath] = md
		}
	}

	// Index server articles
	for i := range serverArticles {
		serverByID[serverArticles[i].ID] = &serverArticles[i]
	}

	// Build article status map
	articleStatus := make(map[string]ArticleStatus)
	articleTitles := make(map[string]string)
	articlePaths := make(map[string]string)

	// Check server articles
	for id, article := range serverByID {
		articleTitles[id] = article.Title
		if localMD, exists := localByID[id]; exists {
			// Article exists locally - check if modified
			localContent := strings.TrimSpace(localMD.Content)
			serverContent := strings.TrimSpace(article.Content)
			if localContent != serverContent {
				articleStatus[id] = StatusModified
			} else {
				articleStatus[id] = StatusUnchanged
			}
			if path, ok := localPaths[id]; ok {
				articlePaths[id] = path
			}
		} else {
			// Article on server but not local
			articleStatus[id] = StatusDeleted
		}
	}

	// Note: New local articles (without ID) will be added to tree later

	// Build tree structure from server articles
	articlesByID := make(map[string]*api.Article)
	for i := range serverArticles {
		articlesByID[serverArticles[i].ID] = &serverArticles[i]
	}

	// Find root articles
	var rootArticles []*api.Article
	for i := range serverArticles {
		if serverArticles[i].ParentID == nil || *serverArticles[i].ParentID == "" {
			rootArticles = append(rootArticles, &serverArticles[i])
		}
	}

	// Sort root articles
	sort.Slice(rootArticles, func(i, j int) bool {
		return rootArticles[i].Order < rootArticles[j].Order
	})

	// Build tree nodes
	var rootNodes []*ArticleNode
	for _, article := range rootArticles {
		node := buildTreeNode(article, articlesByID, articleStatus, articleTitles, articlePaths)
		rootNodes = append(rootNodes, node)
	}

	// Add new local articles (those without IDs) to the tree
	// These should be added based on their file path location
	for path, md := range localByPath {
		// Determine parent from path
		dir := filepath.Dir(path)
		node := &ArticleNode{
			ID:       "",
			Title:    md.Frontmatter.Title,
			Status:   StatusNewLocal,
			Children: []*ArticleNode{},
			Path:     path,
		}

		if dir == "." {
			// Root level new article
			rootNodes = append(rootNodes, node)
		} else {
			// Find parent node by matching path
			parentNode := findNodeByPath(rootNodes, dir)
			if parentNode != nil {
				parentNode.Children = append(parentNode.Children, node)
			} else {
				// Parent not found, add to root
				rootNodes = append(rootNodes, node)
			}
		}
	}

	// Sort root nodes
	sort.Slice(rootNodes, func(i, j int) bool {
		return rootNodes[i].Title < rootNodes[j].Title
	})

	// Display tree
	fmt.Println("\nArticle Tree:")
	displayTree(rootNodes, "", true)

	return nil
}

func buildTreeNode(
	article *api.Article,
	articlesByID map[string]*api.Article,
	articleStatus map[string]ArticleStatus,
	articleTitles map[string]string,
	articlePaths map[string]string,
) *ArticleNode {
	node := &ArticleNode{
		ID:       article.ID,
		Title:    article.Title,
		Status:   articleStatus[article.ID],
		Children: []*ArticleNode{},
	}
	if path, ok := articlePaths[article.ID]; ok {
		node.Path = path
	}

	// Find children
	var children []*api.Article
	for i := range articlesByID {
		child := articlesByID[i]
		if child.ParentID != nil && *child.ParentID == article.ID {
			children = append(children, child)
		}
	}

	// Sort children
	sort.Slice(children, func(i, j int) bool {
		return children[i].Order < children[j].Order
	})

	// Build child nodes
	for _, child := range children {
		childNode := buildTreeNode(child, articlesByID, articleStatus, articleTitles, articlePaths)
		node.Children = append(node.Children, childNode)
	}

	return node
}

func findNodeByPath(nodes []*ArticleNode, targetPath string) *ArticleNode {
	for _, node := range nodes {
		if node.Path != "" {
			// Check if this node's path matches
			dir := filepath.Dir(node.Path)
			if dir == targetPath {
				return node
			}
		}
		// Recursively search children
		if found := findNodeByPath(node.Children, targetPath); found != nil {
			return found
		}
	}
	return nil
}

func displayTree(nodes []*ArticleNode, prefix string, isLast bool) {
	for i, node := range nodes {
		isLastChild := i == len(nodes)-1
		currentPrefix := prefix
		if !isLast {
			currentPrefix += "│   "
		} else {
			currentPrefix += "    "
		}

		// Determine icon based on status
		var icon string
		switch node.Status {
		case StatusUnchanged:
			icon = ""
		case StatusModified:
			icon = "✴️"
		case StatusNewLocal:
			icon = "❇️"
		case StatusDeleted:
			icon = "❌"
		default:
			icon = " "
		}

		// Print current node
		connector := "├── "
		if isLastChild {
			connector = "└── "
		}
		fmt.Printf("%s%s%s %s\n", prefix, connector, icon, node.Title)

		// Recursively display children
		if len(node.Children) > 0 {
			displayTree(node.Children, currentPrefix, isLastChild)
		}
	}
}
