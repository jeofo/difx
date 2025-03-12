# difx

`difx` is a command-line tool that uses Claude AI to explain git diffs. It's a drop-in replacement for the `git diff` command that provides AI-powered explanations of changes.

## Features

- Uses the same syntax as the standard `git diff` command
- Provides AI-powered explanations of code changes
- Gives Claude AI read-only access to your files to provide better context
- Securely stores your Claude API key in `~/.config/difx/config.json`

## Installation

### Prerequisites

- Go 1.21 or higher
- Git
- Claude API key

### Building from source

```bash
git clone https://github.com/jeofo/difx.git
cd difx
go build -o difx
```

Then, move the binary to a location in your PATH:

```bash
sudo mv difx /usr/local/bin/
```

## Usage

```bash
# Basic usage (same as git diff)
difx

# Compare with specific commit
difx HEAD~1

# Compare specific files
difx file1.go file2.go

# Compare branches
difx main feature-branch

# Show only names of changed files
difx --name-only
```

On first run, `difx` will prompt you for your Claude API key, which will be stored in `~/.config/difx/config.json`.

## How it works

1. `difx` runs the standard git diff command with your arguments
2. It sends the diff output to Claude API for analysis
3. Claude analyzes the changes and provides a human-readable explanation
4. The explanation is displayed in your terminal

## Supported Options

`difx` supports most of the standard `git diff` options, including:

- `--stat`: Show a summary of changes
- `--name-only`: Show only the names of changed files
- `--name-status`: Show the names and status of changed files
- `--patch` or `-p`: Generate patch (default)
- `--unified=<n>` or `-U<n>`: Show n lines of context
- `--diff-filter=<filter>`: Filter by added/modified/deleted files

## Troubleshooting

### API Key Issues

If you need to update your Claude API key, you can either:

1. Edit the config file directly at `~/.config/difx/config.json`
2. Delete the config file and run `difx` again to be prompted for a new key
