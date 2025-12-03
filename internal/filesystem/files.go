package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func SanitizeFilename(name string) string {
	invalidChars := "/\\<>:\"|?*"
	result := name
	for _, char := range invalidChars {
		result = strings.ReplaceAll(result, string(char), "_")
	}
	return result
}

func CreateDirectoryStructure(path string) error {
	return os.MkdirAll(path, 0755)
}

func FindMarkdownFiles(basePath string) ([]string, error) {
	var files []string

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			relPath, err := filepath.Rel(basePath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

func GetParentIDFromPath(filePath string, articlesByPath map[string]string) *string {
	dirPath := filepath.Dir(filePath)
	if dirPath == "." || dirPath == "" {
		return nil
	}

	// Find markdown file in parent directory
	parentFile := findMarkdownInDirectory(dirPath)
	if parentFile == "" {
		return nil
	}

	// Read parent file to get ID
	// This is a simplified version - in practice, we'd need to parse the frontmatter
	// For now, we'll return nil and handle it in the calling code
	return nil
}

func findMarkdownInDirectory(dirPath string) string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			return filepath.Join(dirPath, entry.Name())
		}
	}

	return ""
}

func ReadMarkdownFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return string(data), nil
}

func WriteMarkdownFile(filePath string, content string) error {
	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}

