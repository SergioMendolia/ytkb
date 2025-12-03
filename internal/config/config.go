package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/ini.v1"
)

type Config struct {
	Token string
	URL   string
	KBKey string
}

func Load() (*Config, error) {
	cfg := &Config{}

	// Load global config
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	if err := loadConfigFile(configPath, cfg); err != nil {
		if os.IsNotExist(err) {
			if err := createConfigFileInteractive(configPath, cfg); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Load KB_KEY from .env
	if err := loadKBKey(cfg); err != nil {
		if os.IsNotExist(err) {
			if err := selectKnowledgeBaseInteractive(cfg); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return cfg, nil
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "youtrack_writer", "config.ini"), nil
}

func loadConfigFile(path string, cfg *Config) error {
	iniFile, err := ini.Load(path)
	if err != nil {
		return err
	}

	section := iniFile.Section("config")
	cfg.Token = section.Key("token").String()
	cfg.URL = section.Key("url").String()

	if cfg.Token == "" || cfg.URL == "" {
		return fmt.Errorf("invalid config: missing token or url")
	}

	return nil
}

func createConfigFileInteractive(path string, cfg *Config) error {
	fmt.Println("Configuration file not found. Let's set it up!")

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter YouTrack server URL: ")
	url, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read URL: %w", err)
	}
	cfg.URL = strings.TrimSpace(url)

	fmt.Print("Enter API token: ")
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}
	cfg.Token = strings.TrimSpace(token)

	// Create directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create file
	iniFile := ini.Empty()
	section, _ := iniFile.NewSection("config")
	section.NewKey("token", cfg.Token)
	section.NewKey("url", cfg.URL)

	if err := iniFile.SaveTo(path); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	fmt.Printf("Configuration saved to %s\n", path)
	return nil
}

func loadKBKey(cfg *Config) error {
	if err := godotenv.Load(); err != nil {
		return err
	}

	kbKey := os.Getenv("KB_KEY")
	if kbKey == "" {
		return fmt.Errorf("KB_KEY not found in .env")
	}

	cfg.KBKey = kbKey
	return nil
}

func selectKnowledgeBaseInteractive(cfg *Config) error {
	fmt.Println(".env file not found. Let's select a knowledge base.")

	// Try to list knowledge bases
	apiClient := NewAPIClientForKBSelection(cfg)
	bases, err := apiClient.ListKnowledgeBases()
	if err != nil {
		// Fallback to manual input
		fmt.Printf("Could not list knowledge bases: %v\n", err)
		fmt.Println("Please enter KB_KEY manually.")
		return promptForKBKey(cfg)
	}

	if len(bases) == 0 {
		fmt.Println("No knowledge bases found.")
		return promptForKBKey(cfg)
	}

	fmt.Println("\nAvailable knowledge bases:")
	for i, kb := range bases {
		fmt.Printf("  %d. %s (key: %s)\n", i+1, kb.Name, kb.Key)
	}

	fmt.Print("\nEnter number or KB_KEY: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	input = strings.TrimSpace(input)

	// Try to parse as number
	var kbKey string
	if num, err := parseInt(input); err == nil && num > 0 && num <= len(bases) {
		kbKey = bases[num-1].Key
	} else {
		kbKey = input
	}

	cfg.KBKey = kbKey

	// Save to .env
	envContent := fmt.Sprintf("KB_KEY=%s\n", kbKey)
	if err := os.WriteFile(".env", []byte(envContent), 0644); err != nil {
		return fmt.Errorf("failed to save .env file: %w", err)
	}

	fmt.Println("KB_KEY saved to .env")
	return nil
}

func promptForKBKey(cfg *Config) error {
	fmt.Print("Enter Knowledge Base KEY: ")
	reader := bufio.NewReader(os.Stdin)
	kbKey, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read KB_KEY: %w", err)
	}
	kbKey = strings.TrimSpace(kbKey)

	cfg.KBKey = kbKey

	// Save to .env
	envContent := fmt.Sprintf("KB_KEY=%s\n", kbKey)
	if err := os.WriteFile(".env", []byte(envContent), 0644); err != nil {
		return fmt.Errorf("failed to save .env file: %w", err)
	}

	fmt.Println("KB_KEY saved to .env")
	return nil
}

func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// Temporary API client for KB selection (to avoid circular dependency)
type APIClientForKBSelection struct {
	cfg *Config
}

type KnowledgeBase struct {
	Key  string
	Name string
}

func NewAPIClientForKBSelection(cfg *Config) *APIClientForKBSelection {
	return &APIClientForKBSelection{cfg: cfg}
}

func (c *APIClientForKBSelection) ListKnowledgeBases() ([]KnowledgeBase, error) {
	// Use the actual API client
	client := &apiClientWrapper{cfg: c.cfg}
	return client.ListKnowledgeBases()
}

// Wrapper to avoid circular dependency
type apiClientWrapper struct {
	cfg *Config
}

func (c *apiClientWrapper) ListKnowledgeBases() ([]KnowledgeBase, error) {
	// Try /api/knowledgeBases first (for newer YouTrack versions)
	baseURL := strings.TrimSuffix(c.cfg.URL, "/")
	url := fmt.Sprintf("%s/api/knowledgeBases", baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.cfg.Token))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	// If knowledgeBases endpoint works, use it
	if resp.StatusCode == http.StatusOK {
		var bases []KnowledgeBase
		if err := json.NewDecoder(resp.Body).Decode(&bases); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return bases, nil
	}

	// Fallback: Use /api/articles to get unique projects (knowledge bases)
	// This works by getting articles and extracting unique projects
	url = fmt.Sprintf("%s/api/articles?fields=project(id,name)&$top=1000", baseURL)
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.cfg.Token))
	req.Header.Set("Accept", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse articles response to extract unique projects
	type ArticleResponse struct {
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	}

	var articles []ArticleResponse
	if err := json.NewDecoder(resp.Body).Decode(&articles); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract unique projects
	projectMap := make(map[string]string)
	for _, article := range articles {
		if article.Project.ID != "" && article.Project.Name != "" {
			projectMap[article.Project.ID] = article.Project.Name
		}
	}

	// Convert to KnowledgeBase slice
	bases := make([]KnowledgeBase, 0, len(projectMap))
	for id, name := range projectMap {
		bases = append(bases, KnowledgeBase{
			Key:  id,
			Name: name,
		})
	}

	return bases, nil
}
