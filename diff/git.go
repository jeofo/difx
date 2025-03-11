package diff

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// RunGitDiff executes the git diff command with the provided arguments
func RunGitDiff(args []string) (string, error) {
	// Prepare the git diff command
	gitArgs := append([]string{"diff"}, args...)
	
	cmd := exec.Command("git", gitArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		// If there's stderr output, return it as part of the error
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git diff error: %s\n%s", err, stderr.String())
		}
		return "", fmt.Errorf("git diff error: %s", err)
	}
	
	return stdout.String(), nil
}

// GetFileContent retrieves the content of a file at a specific commit
func GetFileContent(filePath string, commitish string) (string, error) {
	if commitish == "" {
		// Read current version
		cmd := exec.Command("cat", filePath)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		
		err := cmd.Run()
		if err != nil {
			if stderr.Len() > 0 {
				return "", fmt.Errorf("error reading file: %s\n%s", err, stderr.String())
			}
			return "", fmt.Errorf("error reading file: %s", err)
		}
		
		return stdout.String(), nil
	}
	
	// Read file at specific commit
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", commitish, filePath))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git show error: %s\n%s", err, stderr.String())
		}
		return "", fmt.Errorf("git show error: %s", err)
	}
	
	return stdout.String(), nil
}

// GetChangedFiles returns a list of files that have been changed
func GetChangedFiles(diffOutput string) []string {
	var files []string
	lines := strings.Split(diffOutput, "\n")
	
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				// Extract the file path from "b/path/to/file"
				filePath := strings.TrimPrefix(parts[3], "b/")
				files = append(files, filePath)
			}
		}
	}
	
	return files
}
