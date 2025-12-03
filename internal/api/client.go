package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"ytkb/internal/config"
)

type Client struct {
	cfg    *config.Config
	client *http.Client
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:    cfg,
		client: &http.Client{},
	}
}

type KnowledgeBase struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type Article struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Content  string  `json:"content"`
	ParentID *string `json:"parentId,omitempty"`
	Order    int     `json:"order"`
	URL      string  `json:"url"`
}

func (c *Client) ListKnowledgeBases() ([]KnowledgeBase, error) {
	// Ensure URL doesn't have trailing slash
	baseURL := strings.TrimSuffix(c.cfg.URL, "/")
	url := fmt.Sprintf("%s/api/knowledgeBases", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.cfg.Token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var bases []KnowledgeBase
	if err := json.NewDecoder(resp.Body).Decode(&bases); err != nil {
		return nil, err
	}

	return bases, nil
}

func (c *Client) ListArticles() ([]Article, error) {
	baseURL := strings.TrimSuffix(c.cfg.URL, "/")

	// Fallback: Use /api/articles and filter client-side
	// The KBKey might be a project ID or project name
	// Fetch all articles and filter by project client-side since query syntax varies
	// Include parent field to preserve hierarchy (try both parentId and parent)
	url := fmt.Sprintf("%s/api/articles?fields=id,summary,content,parentArticle(id),project(id,name)&$top=1000", baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.cfg.Token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse articles response
	type ArticleResponse struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
		Content string `json:"content"`
		Parent  *struct {
			ID string `json:"id"`
		} `json:"parentArticle,omitempty"`
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	}

	var articleResponses []ArticleResponse
	if err := json.NewDecoder(resp.Body).Decode(&articleResponses); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to Article format and filter by KBKey if needed
	articles := make([]Article, 0)
	for _, ar := range articleResponses {
		// If KBKey is specified, only include articles from that project
		// Check both ID and name in case KBKey is a name
		if c.cfg.KBKey != "" {
			if ar.Project.ID != c.cfg.KBKey && ar.Project.Name != c.cfg.KBKey {
				continue
			}
		}

		// Extract parent ID from either parentId field or parent object
		var parentID *string
		if ar.Parent != nil && ar.Parent.ID != "" {
			parentID = &ar.Parent.ID
		}

		article := Article{
			ID:       ar.ID,
			Title:    ar.Summary,
			Content:  ar.Content,
			ParentID: parentID, // Preserve parent relationship
			Order:    0,        // Order might not be available in this endpoint
			URL:      fmt.Sprintf("%s/articles/%s", baseURL, ar.ID),
		}
		articles = append(articles, article)
	}

	return articles, nil
}

func (c *Client) GetArticle(articleID string) (*Article, error) {
	baseURL := strings.TrimSuffix(c.cfg.URL, "/")
	url := fmt.Sprintf("%s/api/knowledgeBases/%s/articles/%s", baseURL, c.cfg.KBKey, articleID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.cfg.Token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var article Article
	if err := json.NewDecoder(resp.Body).Decode(&article); err != nil {
		return nil, err
	}

	return &article, nil
}

func (c *Client) CreateArticle(title, content string, parentID *string) (*Article, error) {
	baseURL := strings.TrimSuffix(c.cfg.URL, "/")
	url := fmt.Sprintf("%s/api/articles", baseURL)

	fmt.Printf("Creating article: %s\n", title)
	payload := map[string]interface{}{
		"summary": title,
		"content": content,
		"project": map[string]interface{}{
			"id": c.cfg.KBKey,
		},
	}
	if parentID != nil {
		payload["parentId"] = *parentID
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.cfg.Token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var article Article
	if err := json.NewDecoder(resp.Body).Decode(&article); err != nil {
		return nil, err
	}

	return &article, nil
}

func (c *Client) UpdateArticle(articleID, title, content string) (*Article, error) {
	baseURL := strings.TrimSuffix(c.cfg.URL, "/")
	url := fmt.Sprintf("%s/api/articles/%s", baseURL, articleID)

	payload := map[string]interface{}{
		"title":   title,
		"content": content,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.cfg.Token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var article Article
	if err := json.NewDecoder(resp.Body).Decode(&article); err != nil {
		return nil, err
	}

	return &article, nil
}
