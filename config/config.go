package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the application configuration
type Config struct {
	ClaudeAPIKey string `json:"claude_api_key"`
}

// ConfigDir is the directory where config is stored
const ConfigDir = "~/.config/claudiff"

// ConfigFile is the path to the config file
const ConfigFile = "config.json"

// expandPath expands the tilde in the path to the user's home directory
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// getConfigPath returns the full path to the config file
func getConfigPath() (string, error) {
	expandedDir, err := expandPath(ConfigDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(expandedDir, ConfigFile), nil
}

// LoadOrCreate loads the config file if it exists, or creates a new one if it doesn't
func LoadOrCreate() (*Config, error) {
	expandedDir, err := expandPath(ConfigDir)
	if err != nil {
		return nil, err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(expandedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create empty config
		return &Config{}, nil
	}

	// Read config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return &config, nil
}

// Save saves the config to disk
func Save(config *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// PromptForAPIKey prompts the user to enter their Claude API key
func PromptForAPIKey() (string, error) {
	fmt.Print("Please enter your Claude API key: ")
	reader := bufio.NewReader(os.Stdin)
	apiKey, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read API key: %w", err)
	}

	// Trim whitespace and newlines
	apiKey = strings.TrimSpace(apiKey)

	return apiKey, nil
}
