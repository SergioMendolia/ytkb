package markdown

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Frontmatter struct {
	ID    string `yaml:"id,omitempty"`
	Title string `yaml:"title"`
	URL   string `yaml:"url,omitempty"`
}

type MarkdownFile struct {
	Frontmatter Frontmatter
	Content     string
}

func ParseMarkdown(content string) (*MarkdownFile, error) {
	// Find frontmatter delimiters
	frontmatterStart := strings.Index(content, "---\n")
	if frontmatterStart == -1 {
		return nil, fmt.Errorf("no frontmatter found")
	}

	frontmatterEnd := strings.Index(content[frontmatterStart+4:], "---\n")
	if frontmatterEnd == -1 {
		return nil, fmt.Errorf("invalid frontmatter: no closing delimiter")
	}

	frontmatterText := content[frontmatterStart+4 : frontmatterStart+4+frontmatterEnd]
	bodyStart := frontmatterStart + 4 + frontmatterEnd + 4

	var frontmatter Frontmatter
	if err := yaml.Unmarshal([]byte(frontmatterText), &frontmatter); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	body := ""
	if bodyStart < len(content) {
		body = content[bodyStart:]
	}

	return &MarkdownFile{
		Frontmatter: frontmatter,
		Content:     body,
	}, nil
}

func WriteMarkdown(fm Frontmatter, content string) (string, error) {
	var builder strings.Builder

	builder.WriteString("---\n")

	// Write YAML frontmatter
	yamlData, err := yaml.Marshal(&fm)
	if err != nil {
		return "", err
	}
	builder.Write(yamlData)

	builder.WriteString("---\n")
	builder.WriteString(content)

	return builder.String(), nil
}

func UpdateFrontmatterID(filePath string, articleID string) error {
	// Read file
	content, err := readFile(filePath)
	if err != nil {
		return err
	}

	// Parse
	md, err := ParseMarkdown(content)
	if err != nil {
		return err
	}

	// Update ID
	md.Frontmatter.ID = articleID

	// Write back
	newContent, err := WriteMarkdown(md.Frontmatter, md.Content)
	if err != nil {
		return err
	}

	return writeFile(filePath, newContent)
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func writeFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

