package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tydin/claudiff/config"
	"github.com/tydin/claudiff/diff"
)

var rootCmd = &cobra.Command{
	Use:   "claudiff [options] [--] [<path>...]",
	Short: "A tool that uses Claude AI to explain git diffs",
	Long: `claudiff is a command-line tool that uses Claude AI to explain git diffs.
It accepts the same syntax as the git diff command and provides AI-powered explanations.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load or create config
		cfg, err := config.LoadOrCreate()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %s\n", err)
			os.Exit(1)
		}

		// Check if API key is available
		if cfg.ClaudeAPIKey == "" {
			apiKey, err := config.PromptForAPIKey()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting API key: %s\n", err)
				os.Exit(1)
			}
			cfg.ClaudeAPIKey = apiKey
			if err := config.Save(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %s\n", err)
				os.Exit(1)
			}
		}

		// Process git diff and get explanation
		diffOutput, err := diff.RunGitDiff(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running git diff: %s\n", err)
			os.Exit(1)
		}

		if diffOutput == "" {
			fmt.Println("No differences found.")
			return
		}

		// Create a channel for streaming output
		outputChan := make(chan string)
		
		// Start a goroutine to handle the display of streaming output
		go func() {
			var buffer strings.Builder
			var lastProcessed string
			
			for chunk := range outputChan {
				// Add the new chunk to the buffer
				buffer.WriteString(chunk)
				
				// Get the current full text
				currentText := buffer.String()
				
				// Clean up any incomplete escape sequences at the end of the text
				currentText = cleanIncompleteEscapeSequences(currentText)
				
				// Convert \033 escape sequences to actual escape characters
				processedText := convertEscapeSequences(currentText)
				
				// Only print the new part (what's been added since last time)
				if len(lastProcessed) < len(processedText) {
					newPart := processedText[len(lastProcessed):]
					fmt.Printf("%s", newPart) // Use Printf for better handling of escape sequences
					lastProcessed = processedText
				}
			}
			
			// Print a final newline when done
			fmt.Println()
		}()
		
		// Create a callback function to process streaming output
		streamCallback := func(chunk string) {
			outputChan <- chunk
		}

		// Call the API with streaming callback
		_, err = diff.GetExplanation(diffOutput, cfg.ClaudeAPIKey, streamCallback)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError getting explanation from Claude: %s\n", err)
			os.Exit(1)
		}
		
		// Close the output channel to signal completion
		close(outputChan)
	},
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

// convertEscapeSequences converts \033 escape sequences to actual escape characters
func convertEscapeSequences(text string) string {
	// Replace \033 with the actual escape character
	result := strings.ReplaceAll(text, "\\033", "\033")
	
	// For backward compatibility, also handle the old markers
	// Create color objects
	red := color.New(color.FgRed, color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	
	// Find and replace additions (green text) with [ADD] markers
	addRegex := regexp.MustCompile(`\[ADD\](.*?)\[/ADD\]`)
	result = addRegex.ReplaceAllStringFunc(result, func(match string) string {
		submatches := addRegex.FindStringSubmatch(match)
		if len(submatches) > 1 {
			return green.Sprint(submatches[1])
		}
		return match
	})
	
	// Find and replace deletions (red text) with [DEL] markers
	delRegex := regexp.MustCompile(`\[DEL\](.*?)\[/DEL\]`)
	result = delRegex.ReplaceAllStringFunc(result, func(match string) string {
		submatches := delRegex.FindStringSubmatch(match)
		if len(submatches) > 1 {
			return red.Sprint(submatches[1])
		}
		return match
	})
	
	// Also handle the GREEN_START/GREEN_END and RED_START/RED_END markers for backward compatibility
	greenRegex := regexp.MustCompile(`GREEN_START(.*?)GREEN_END`)
	result = greenRegex.ReplaceAllStringFunc(result, func(match string) string {
		submatches := greenRegex.FindStringSubmatch(match)
		if len(submatches) > 1 {
			return green.Sprint(submatches[1])
		}
		return match
	})
	
	redRegex := regexp.MustCompile(`RED_START(.*?)RED_END`)
	result = redRegex.ReplaceAllStringFunc(result, func(match string) string {
		submatches := redRegex.FindStringSubmatch(match)
		if len(submatches) > 1 {
			return red.Sprint(submatches[1])
		}
		return match
	})
	
	return result
}

// cleanIncompleteEscapeSequences removes incomplete escape sequences at the end of text
// This helps when an escape sequence is split across multiple chunks
func cleanIncompleteEscapeSequences(text string) string {
	// Check for incomplete \033 escape sequence at the end
	if strings.HasSuffix(text, "\\") {
		return text[:len(text)-1]
	}
	if strings.HasSuffix(text, "\\0") {
		return text[:len(text)-2]
	}
	if strings.HasSuffix(text, "\\03") {
		return text[:len(text)-3]
	}
	if strings.HasSuffix(text, "\\033") {
		return text[:len(text)-4]
	}
	if strings.HasSuffix(text, "\\033[") {
		return text[:len(text)-5]
	}
	if strings.HasSuffix(text, "\\033[3") {
		return text[:len(text)-6]
	}
	if strings.HasSuffix(text, "\\033[32") {
		return text[:len(text)-7]
	}
	if strings.HasSuffix(text, "\\033[32;") {
		return text[:len(text)-8]
	}
	if strings.HasSuffix(text, "\\033[32;1") {
		return text[:len(text)-9]
	}
	if strings.HasSuffix(text, "\\033[31") {
		return text[:len(text)-7]
	}
	if strings.HasSuffix(text, "\\033[31;") {
		return text[:len(text)-8]
	}
	if strings.HasSuffix(text, "\\033[31;1") {
		return text[:len(text)-9]
	}
	
	// For backward compatibility, also check for incomplete markers
	// Check for incomplete [ADD]/[DEL] markers
	if strings.HasSuffix(text, "[") {
		return text[:len(text)-1]
	}
	if strings.HasSuffix(text, "[A") {
		return text[:len(text)-2]
	}
	if strings.HasSuffix(text, "[AD") {
		return text[:len(text)-3]
	}
	if strings.HasSuffix(text, "[ADD") {
		return text[:len(text)-4]
	}
	if strings.HasSuffix(text, "[D") {
		return text[:len(text)-2]
	}
	if strings.HasSuffix(text, "[DE") {
		return text[:len(text)-3]
	}
	if strings.HasSuffix(text, "[DEL") {
		return text[:len(text)-4]
	}
	
	// Check for incomplete GREEN_START/RED_START markers
	if strings.HasSuffix(text, "G") {
		return text[:len(text)-1]
	}
	if strings.HasSuffix(text, "GR") {
		return text[:len(text)-2]
	}
	if strings.HasSuffix(text, "GREEN_START") {
		return text[:len(text)-11]
	}
	if strings.HasSuffix(text, "R") {
		return text[:len(text)-1]
	}
	if strings.HasSuffix(text, "RE") {
		return text[:len(text)-2]
	}
	if strings.HasSuffix(text, "RED_START") {
		return text[:len(text)-9]
	}
	
	return text
}

func init() {
	// Force color output regardless of terminal detection
	color.NoColor = false
	
	// Add flags that git diff supports
	rootCmd.Flags().BoolP("patch", "p", true, "Generate patch")
	rootCmd.Flags().BoolP("stat", "", false, "Generate diffstat")
	rootCmd.Flags().BoolP("name-only", "", false, "Show only names of changed files")
	rootCmd.Flags().BoolP("name-status", "", false, "Show only names and status of changed files")
	rootCmd.Flags().StringP("diff-filter", "", "", "Filter by added/modified/deleted")
	rootCmd.Flags().StringP("unified", "U", "", "Show n lines of context")
	
	// Add claudiff specific flags
	rootCmd.Flags().BoolP("verbose", "v", false, "Show detailed output including the diff")
}
