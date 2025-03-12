package cmd

import (
	"fmt"
	"os"
	"regexp"

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

		explanation, err := diff.GetExplanation(diffOutput, cfg.ClaudeAPIKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting explanation from Claude: %s\n", err)
			os.Exit(1)
		}

		// Process the explanation to apply terminal colors
		coloredExplanation := processColors(explanation)
		fmt.Println(coloredExplanation)
	},
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

// processColors replaces [ADD]/[DEL] markers with colored text using the fatih/color package
func processColors(text string) string {
	// Create color objects
	red := color.New(color.FgRed, color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	
	// Find and replace additions (green text)
	addRegex := regexp.MustCompile(`\[ADD\](.*?)\[/ADD\]`)
	result := addRegex.ReplaceAllStringFunc(text, func(match string) string {
		// Extract the text between the markers
		submatches := addRegex.FindStringSubmatch(match)
		if len(submatches) > 1 {
			return green.Sprint(submatches[1])
		}
		return match
	})
	
	// Find and replace deletions (red text)
	delRegex := regexp.MustCompile(`\[DEL\](.*?)\[/DEL\]`)
	result = delRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the text between the markers
		submatches := delRegex.FindStringSubmatch(match)
		if len(submatches) > 1 {
			return red.Sprint(submatches[1])
		}
		return match
	})
	
	return result
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
