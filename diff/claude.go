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
	ClaudeModel  = "claude-3-7-sonnet-latest"
)

// ClaudeRequest represents the request structure for the Claude API
type ClaudeRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
}

// Message represents a message in the Claude API request
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeResponse represents the response structure from the Claude API
type ClaudeResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
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
	prompt += "Here's the git diff output:\n\n```\n"
	prompt += diffOutput
	prompt += "\n```\n\n"
	prompt += "Please consider:\n"
	prompt += "1. What files were changed\n"
	prompt += "2. The purpose and impact of the changes\n"
	prompt += "3. Any potential issues or considerations\n"
	prompt += "I want you to be concise (less than 200 words) using the format below, do not return it in ```, return the text only:\n\n```"
	prompt += `
--------------------------------------------------
                  CLAUDIFF
--------------------------------------------------
SUMMARY:
  - Files modified: {files_modified}
  - Insertions: {insertions}
  - Deletions: {deletions}

FILE CHANGES:
{file_changes}

DETAILED BREAKDOWN:
	file1:
		- {detailed_breakdown}
	file2:
		- {detailed_breakdown}
--------------------------------------------------
`
	prompt += "\n```\n"
	prompt += "Instead of using ANSI color codes directly, please use the following special markers to indicate text that should be colored:\n\n"
	prompt += "For additions (green text): [ADD]your text here[/ADD]\n"
	prompt += "For deletions (red text): [DEL]your text here[/DEL]\n\n"
	prompt += "IMPORTANT: Make sure to use these exact markers. Do not use ANSI escape codes or any other formatting."

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
