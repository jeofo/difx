package diff

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tydin/difx/config"
)

const (
	// Claude API constants
	ClaudeAPIURL = "https://api.anthropic.com/v1/messages"
	ClaudeModel  = "claude-3-7-sonnet-latest"
	
	// Azure OpenAI constants
	AzureOpenAIModel = "gpt-4o"
	AzureOpenAIAPIVersion = "2024-02-15-preview"
)

// ClaudeRequest represents the request structure for the Claude API
type ClaudeRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream"`
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

// Event types for streaming response
const (
	EventMessageStart      = "message_start"
	EventMessageDelta      = "message_delta"
	EventMessageStop       = "message_stop"
	EventContentBlockStart = "content_block_start"
	EventContentBlockDelta = "content_block_delta"
	EventContentBlockStop  = "content_block_stop"
	EventPing              = "ping"
)

// StreamEvent represents a streaming event from Claude API
type StreamEvent struct {
	Type         string         `json:"type"`
	Message      *StreamMessage `json:"message,omitempty"`
	Delta        *StreamDelta   `json:"delta,omitempty"`
	Index        int            `json:"index,omitempty"`
	ContentBlock *ContentBlock  `json:"content_block,omitempty"`
}

// StreamMessage represents the message in a streaming response
type StreamMessage struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
}

// StreamDelta represents the delta in a streaming response
type StreamDelta struct {
	Type         string  `json:"type,omitempty"`
	Text         string  `json:"text,omitempty"`
	StopReason   *string `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
}

// GetExplanation sends the diff to the selected LLM API and returns an explanation
func GetExplanation(diffOutput string, cfg *config.Config, callback func(string)) (string, error) {
	// Create the prompt for Claude
	prompt := "I'm going to show you the output of a git diff command. Please explain these changes in a clear, concise way.\n\n"
	prompt += "Here's the git diff output:\n\n```\n"
	prompt += diffOutput
	prompt += "\n```\n\n"
	prompt += "Be concise but include every file that was changed in DETAILS. Use the format below and output plaintext without ```. Only include SUMMARY,FILE CHANGES and DETAILS section:\n\n```"
	prompt += `
--------------------------------------------------
SUMMARY:
  - Files modified: {files_modified}
	- One line summary of the changes
  - Insertions: {insertions}
  - Deletions: {deletions}

FILE CHANGES:
{file_changes}

DETAILS:
	file1:
		+ {detailed_breakdown_additions}
		- {detailed_breakdown_deletions}
	...
--------------------------------------------------
`
	prompt += "\n```\n"
	prompt += "IMPORTANT: For colored text, use the following ANSI escape codes with the full escape character prefix:\n\n"
	prompt += "For additions (green text): \\033[32;1m text here \\033[0m\n"
	prompt += "For deletions (red text): \\033[31;1m text here \\033[0m\n\n"
	prompt += "Make sure to include the full '\\033' escape character prefix and always close with '\\033[0m' to reset the color."

	// Determine which model to use based on the active model in config
	switch cfg.ActiveModel {
	case config.ModelClaude:
		return callClaudeAPI(prompt, cfg, callback)
	case config.ModelAzureOpenAI:
		return callAzureOpenAI(prompt, cfg, callback)
	default:
		return "", fmt.Errorf("unsupported model: %s", cfg.ActiveModel)
	}
}

// callClaudeAPI sends the prompt to Claude API and returns the response
func callClaudeAPI(prompt string, cfg *config.Config, callback func(string)) (string, error) {
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
		Stream:      cfg.Streaming,
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
	req.Header.Set("x-api-key", cfg.ClaudeAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	
	// Handle streaming vs non-streaming
	if cfg.Streaming {
		req.Header.Set("Accept", "text/event-stream")
		return handleClaudeStreamingResponse(req, callback)
	} else {
		return handleClaudeNonStreamingResponse(req)
	}
}

// handleClaudeStreamingResponse processes a streaming response from Claude API
func handleClaudeStreamingResponse(req *http.Request, callback func(string)) (string, error) {
	// Create a channel to receive the streamed content
	contentChan := make(chan string)
	errChan := make(chan error)

	// Start a goroutine to process the streaming response
	go func() {
		// Send the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			errChan <- fmt.Errorf("error sending request to Claude API: %w", err)
			return
		}
		defer resp.Body.Close()

		// Check for non-200 status code
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("Claude API returned non-200 status code: %d, body: %s", resp.StatusCode, string(respBody))
			return
		}

		// Create a scanner to read the SSE stream line by line
		scanner := bufio.NewScanner(resp.Body)
		var eventType string
		var eventData string

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			// Parse the event type
			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
				continue
			}

			// Parse the event data
			if strings.HasPrefix(line, "data: ") {
				eventData = strings.TrimPrefix(line, "data: ")

				// Skip ping events
				if eventType == EventPing {
					continue
				}

				// Parse the event data
				var streamEvent StreamEvent
				if err := json.Unmarshal([]byte(eventData), &streamEvent); err != nil {
					errChan <- fmt.Errorf("error unmarshalling stream event: %w, data: %s", err, eventData)
					return
				}

				// Process the event based on its type
				switch eventType {
				case EventMessageStart:
					// Message started, nothing to do yet

				case EventContentBlockStart:
					// Content block started, nothing to do yet
					// If it's a text block, we might want to add a newline
					if streamEvent.ContentBlock != nil && streamEvent.ContentBlock.Type == "text" {
						// Optional: Add a newline before new content blocks
						// contentChan <- "\n"
						// if callback != nil {
						//     callback("\n")
						// }
					}

				case EventContentBlockDelta:
					// Check if this is a text delta
					if streamEvent.Delta != nil && streamEvent.Delta.Type == "text_delta" {
						text := streamEvent.Delta.Text
						if text != "" {
							// Send the text delta to the channel
							contentChan <- text

							// Call the callback function with the new content
							if callback != nil {
								callback(text)
							}
						}
					}

				case EventContentBlockStop:
					// Content block stopped, nothing to do

				case EventMessageDelta:
					// Message delta received, check if it has a stop reason
					if streamEvent.Delta != nil && streamEvent.Delta.StopReason != nil {
						// The message is complete
					}

				case EventMessageStop:
					// Message stopped, close the channel
					close(contentChan)
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("error reading stream: %w", err)
		}
	}()

	// Collect the streamed content
	var fullResponse strings.Builder
	for {
		select {
		case content, ok := <-contentChan:
			if !ok {
				// Channel closed, streaming is complete
				return strings.TrimSpace(fullResponse.String()), nil
			}
			fullResponse.WriteString(content)
		case err := <-errChan:
			return "", err
		}
	}
}

// handleClaudeNonStreamingResponse processes a non-streaming response from Claude API
func handleClaudeNonStreamingResponse(req *http.Request) (string, error) {
	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request to Claude API: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Claude API returned non-200 status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	// Parse the response
	var claudeResp ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return "", fmt.Errorf("error decoding Claude API response: %w", err)
	}

	// Extract the text from the response
	if len(claudeResp.Content) > 0 && claudeResp.Content[0].Type == "text" {
		return claudeResp.Content[0].Text, nil
	}

	return "", fmt.Errorf("no text content found in Claude API response")
}

// AzureOpenAIRequest represents the request structure for the Azure OpenAI API
type AzureOpenAIRequest struct {
	Messages    []AzureOpenAIMessage `json:"messages"`
	Temperature float64              `json:"temperature"`
	TopP        float64              `json:"top_p"`
	MaxTokens   int                  `json:"max_tokens"`
	Stream      bool                 `json:"stream"`
}

// AzureOpenAIMessage represents a message in the Azure OpenAI API request
type AzureOpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AzureOpenAIResponse represents the response structure from the Azure OpenAI API
type AzureOpenAIResponse struct {
	ID      string                    `json:"id"`
	Object  string                    `json:"object"`
	Created int64                     `json:"created"`
	Model   string                    `json:"model"`
	Choices []AzureOpenAIResponseChoice `json:"choices"`
}

// AzureOpenAIResponseChoice represents a choice in the Azure OpenAI API response
type AzureOpenAIResponseChoice struct {
	Index        int                      `json:"index"`
	Message      AzureOpenAIMessage       `json:"message"`
	FinishReason string                   `json:"finish_reason"`
}

// AzureOpenAIStreamResponse represents a streaming response from Azure OpenAI API
type AzureOpenAIStreamResponse struct {
	ID      string                    `json:"id"`
	Object  string                    `json:"object"`
	Created int64                     `json:"created"`
	Model   string                    `json:"model"`
	Choices []AzureOpenAIStreamChoice `json:"choices"`
}

// AzureOpenAIStreamChoice represents a choice in a streaming response
type AzureOpenAIStreamChoice struct {
	Index        int                 `json:"index"`
	Delta        AzureOpenAIDelta    `json:"delta"`
	FinishReason string              `json:"finish_reason"`
}

// AzureOpenAIDelta represents the delta in a streaming response
type AzureOpenAIDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// callAzureOpenAI sends the prompt to Azure OpenAI API and returns the response
func callAzureOpenAI(prompt string, cfg *config.Config, callback func(string)) (string, error) {
	// Create the request for Azure OpenAI
	request := AzureOpenAIRequest{
		Messages: []AzureOpenAIMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7,
		TopP:        0.95,
		MaxTokens:   4000,
		Stream:      cfg.Streaming,
	}

	// Convert request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("error marshalling request: %w", err)
	}

	// Create the URL for Azure OpenAI API
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", 
		cfg.AzureOpenAIEndpoint, 
		AzureOpenAIModel, 
		AzureOpenAIAPIVersion)

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("error creating HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", cfg.AzureOpenAIKey)

	// Handle streaming vs non-streaming
	if cfg.Streaming {
		return handleAzureOpenAIStreamingResponse(req, callback)
	} else {
		return handleAzureOpenAINonStreamingResponse(req)
	}
}

// handleAzureOpenAIStreamingResponse processes a streaming response from Azure OpenAI API
func handleAzureOpenAIStreamingResponse(req *http.Request, callback func(string)) (string, error) {
	// Add streaming header
	req.Header.Set("Accept", "text/event-stream")

	// Create a channel to receive the streamed content
	contentChan := make(chan string)
	errChan := make(chan error)

	// Start a goroutine to process the streaming response
	go func() {
		// Send the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			errChan <- fmt.Errorf("error sending request to Azure OpenAI API: %w", err)
			return
		}
		defer resp.Body.Close()

		// Check for non-200 status code
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("Azure OpenAI API returned non-200 status code: %d, body: %s", resp.StatusCode, string(respBody))
			return
		}

		// Create a scanner to read the SSE stream line by line
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			// Parse the event data
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				// Check for [DONE] message
				if data == "[DONE]" {
					close(contentChan)
					return
				}

				// Parse the data as JSON
				var streamResp AzureOpenAIStreamResponse
				if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
					errChan <- fmt.Errorf("error unmarshalling stream response: %w, data: %s", err, data)
					return
				}

				// Process the choices
				for _, choice := range streamResp.Choices {
					if choice.Delta.Content != "" {
						// Send the content delta to the channel
						contentChan <- choice.Delta.Content

						// Call the callback function with the new content
						if callback != nil {
							callback(choice.Delta.Content)
						}
					}

					// Check if we're done
					if choice.FinishReason != "" {
						close(contentChan)
						return
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("error reading stream: %w", err)
		}
	}()

	// Collect the streamed content
	var fullResponse strings.Builder
	for {
		select {
		case content, ok := <-contentChan:
			if !ok {
				// Channel closed, streaming is complete
				return strings.TrimSpace(fullResponse.String()), nil
			}
			fullResponse.WriteString(content)
		case err := <-errChan:
			return "", err
		}
	}
}

// handleAzureOpenAINonStreamingResponse processes a non-streaming response from Azure OpenAI API
func handleAzureOpenAINonStreamingResponse(req *http.Request) (string, error) {
	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request to Azure OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Azure OpenAI API returned non-200 status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	// Parse the response
	var azureResp AzureOpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&azureResp); err != nil {
		return "", fmt.Errorf("error decoding Azure OpenAI API response: %w", err)
	}

	// Extract the text from the response
	if len(azureResp.Choices) > 0 {
		return azureResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no content found in Azure OpenAI API response")
}
