package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/clicky"
)

// CommitInfo represents a git commit to analyze
type CommitInfo struct {
	Hash        string
	Message     string
	Author      string
	Date        time.Time
	FileChanges []FileChange
	Patch       string
}

// FileChange represents a file change in a commit
type FileChange struct {
	Path       string
	ChangeType string
	Additions  int
	Deletions  int
}

// CommitAnalyzer analyzes git commits using Claude
type CommitAnalyzer struct {
	executor *ClaudeExecutor
	options  AnalyzerOptions
}

// AnalyzerOptions configures the commit analyzer
type AnalyzerOptions struct {
	ClaudeOptions
	PromptTemplate string
	CacheDir       string
}

// NewCommitAnalyzer creates a new commit analyzer
func NewCommitAnalyzer(options AnalyzerOptions) *CommitAnalyzer {
	executor := NewClaudeExecutor(options.ClaudeOptions)
	
	if options.PromptTemplate == "" {
		options.PromptTemplate = defaultPromptTemplate
	}
	
	return &CommitAnalyzer{
		executor: executor,
		options:  options,
	}
}

// AnalyzeCommit analyzes a single commit
func (ca *CommitAnalyzer) AnalyzeCommit(ctx context.Context, commit CommitInfo) (string, error) {
	prompt := ca.buildPrompt(commit)
	
	task := ca.executor.taskManager.Start(
		fmt.Sprintf("Analyzing %s", commit.Hash[:8]),
		clicky.WithTimeout(2*time.Minute),
		clicky.WithFunc(func(t *clicky.Task) error {
			t.Infof("Commit by %s: %s", commit.Author, firstLine(commit.Message))
			t.SetProgress(10, 100)
			
			response, err := ca.executor.executeClaudeCLI(t.Context(), prompt, t)
			if err != nil {
				return fmt.Errorf("failed to analyze: %w", err)
			}
			
			t.SetProgress(100, 100)
			t.Infof("Analysis complete (%d tokens)", response.GetTotalTokens())
			
			// Store result for retrieval
			commit.Hash = response.Result // Hack: store result in hash field
			return nil
		}),
	)
	
	// Wait for completion
	for task.Status() == clicky.StatusPending || task.Status() == clicky.StatusRunning {
		select {
		case <-ctx.Done():
			task.Cancel()
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	
	if err := task.Error(); err != nil {
		return "", err
	}
	
	// Get the actual response
	response, err := ca.executor.executeClaudeCLI(ctx, prompt, nil)
	if err != nil {
		return "", err
	}
	
	return ca.cleanResponse(response.Result), nil
}

// AnalyzeCommitsBatch analyzes multiple commits in parallel
func (ca *CommitAnalyzer) AnalyzeCommitsBatch(ctx context.Context, commits []CommitInfo) (map[string]string, error) {
	prompts := make(map[string]string)
	commitMap := make(map[string]CommitInfo)
	
	for _, commit := range commits {
		key := fmt.Sprintf("%s (%s)", commit.Hash[:8], firstLine(commit.Message))
		prompts[key] = ca.buildPrompt(commit)
		commitMap[key] = commit
	}
	
	// Use the executor's batch processing
	responses, err := ca.executor.ExecutePromptBatch(ctx, prompts)
	if err != nil {
		return nil, err
	}
	
	// Map responses back to commit hashes
	results := make(map[string]string)
	for key, response := range responses {
		if commit, ok := commitMap[key]; ok {
			results[commit.Hash] = ca.cleanResponse(response.Result)
		}
	}
	
	return results, nil
}

// GetTaskManager returns the underlying task manager for monitoring
func (ca *CommitAnalyzer) GetTaskManager() *clicky.TaskManager {
	return ca.executor.taskManager
}

// buildPrompt builds the analysis prompt for a commit
func (ca *CommitAnalyzer) buildPrompt(commit CommitInfo) string {
	var sb strings.Builder
	
	if ca.options.PromptTemplate != "" {
		sb.WriteString(ca.options.PromptTemplate)
		sb.WriteString("\n\n")
	}
	
	sb.WriteString(fmt.Sprintf("Commit: %s\n", commit.Hash))
	sb.WriteString(fmt.Sprintf("Author: %s\n", commit.Author))
	sb.WriteString(fmt.Sprintf("Date: %s\n", commit.Date.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("Message: %s\n\n", commit.Message))
	
	// Count changes
	additions := 0
	deletions := 0
	for _, fc := range commit.FileChanges {
		additions += fc.Additions
		deletions += fc.Deletions
	}
	
	sb.WriteString(fmt.Sprintf("Files changed: %d files (%d additions, %d deletions)\n\n",
		len(commit.FileChanges), additions, deletions))
	
	// List changed files
	if len(commit.FileChanges) > 0 {
		sb.WriteString("Changed files:\n")
		for _, fc := range commit.FileChanges {
			sb.WriteString(fmt.Sprintf("- %s (%s) +%d -%d\n",
				fc.Path, fc.ChangeType, fc.Additions, fc.Deletions))
		}
		sb.WriteString("\n")
	}
	
	// Add patch if available
	if commit.Patch != "" {
		sb.WriteString("Patch diff:\n")
		sb.WriteString(commit.Patch)
		sb.WriteString("\n\n")
	}
	
	sb.WriteString("Provide a concise one-line summary of this commit for a changelog. Focus on the user-facing impact and benefits.")
	
	return sb.String()
}

// cleanResponse cleans up the Claude response
func (ca *CommitAnalyzer) cleanResponse(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "This commit ")
	response = strings.TrimPrefix(response, "The commit ")
	return response
}

// firstLine returns the first line of a multi-line string
func firstLine(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 {
		line := lines[0]
		if len(line) > 50 {
			return line[:47] + "..."
		}
		return line
	}
	return s
}

const defaultPromptTemplate = `Analyze this git commit and provide a concise technical summary suitable for a changelog.
Focus on:
- What changed from a user perspective
- Why it matters
- Any breaking changes or important notes

Do not reference internal implementation details or code structure unless they directly impact users.`