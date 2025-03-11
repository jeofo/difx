package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	ClaudeAPIURL = "https://api.anthropic.com/v1/messages"
	ClaudeModel  = "claude-3-opus-20240229"
)

// ClaudeRequest represents the request structure for the Claude API
type ClaudeRequest struct {
	Model       string   `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int      `json:"max_tokens"`
	Temperature float64  `json:"temperature,omitempty"`
}

// Message represents a message in the Claude API request
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeResponse represents the response structure from the Claude API
type ClaudeResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Role       string `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
}

// ContentBlock represents a block of content in the Claude API response
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// GetExplanation sends the diff to Claude API and returns an explanation
func GetExplanation(diffOutput string, apiKey string) (string, error) {
	// Create the prompt for Claude
	prompt := "I'm going to show you the output of a git diff command. Please explain these changes in a clear, concise way.\n\n"
	prompt += "If you need more context about the code, you can ask me to show you the content of specific files or the content of files at specific commits.\n\n"
	prompt += "Here's the git diff output:\n\n```\n"
	prompt += diffOutput
	prompt += "\n```\n\n"
	prompt += "Please explain:\n"
	prompt += "1. What files were changed\n"
	prompt += "2. The purpose and impact of the changes\n"
	prompt += "3. Any potential issues or considerations\n"
	prompt += "4. A summary of the overall change"

	// Create the request for Claude
	request := ClaudeRequest{
		Model: ClaudeModel,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4000,
		Temperature: 0.7,
	}

	// Convert request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("error marshalling request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", ClaudeAPIURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("error creating HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request to Claude API: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Claude API returned non-200 status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	// Parse the response
	var claudeResponse ClaudeResponse
	err = json.Unmarshal(respBody, &claudeResponse)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling response: %w", err)
	}

	// Extract the text from the response
	var responseText strings.Builder
	for _, block := range claudeResponse.Content {
		if block.Type == "text" {
			responseText.WriteString(block.Text)
		}
	}

	return strings.TrimSpace(responseText.String()), nil
}
