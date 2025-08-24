package clicky

import (
	"fmt"
	"sync"
	"time"

	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
)

// Dependency represents a scanned dependency
type Dependency struct {
	GitURL     string
	Tag        string
	Hash       string
	Modules    []Module
	ScannedAt  time.Time
	CachedAt   *time.Time // When it was cached (nil if not from cache)
}

// Module represents a module within a dependency
type Module struct {
	Name         string
	Version      string
	Dependencies []string
}

// DependencyScanner manages dependency scanning with deduplication and caching
type DependencyScanner struct {
	manager       *task.Manager
	cache         *DependencyCache
	scanningTasks sync.Map // Track active scans by "gitURL@tag"
}

// NewDependencyScanner creates a new dependency scanner
func NewDependencyScanner(manager *task.Manager, cache *DependencyCache) *DependencyScanner {
	return &DependencyScanner{
		manager: manager,
		cache:   cache,
	}
}

// ScanDependency scans a dependency, using cache and deduplication
func (s *DependencyScanner) ScanDependency(gitURL, tag string) *task.Task {
	identity := fmt.Sprintf("dep-scan:%s@%s", gitURL, tag)
	
	// Check if task already exists in scanningTasks (for immediate return)
	if existing, ok := s.scanningTasks.Load(identity); ok {
		return existing.(*task.Task)
	}
	
	// Check cache first
	if s.cache != nil {
		if cached, err := s.cache.Get(gitURL, tag); err == nil && cached != nil {
			// Create a completed task with cached data
			return s.createCompletedTask(cached)
		}
	}
	
	// Create new scan task with identity for deduplication
	scanTask := s.manager.Start(
		fmt.Sprintf("Scanning %s@%s", gitURL, tag),
		task.WithIdentity(identity),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Starting dependency scan for %s@%s", gitURL, tag)
			t.SetProgress(0, 0) // Indeterminate progress
			
			// Perform actual scanning
			dep, err := s.performScan(ctx, gitURL, tag, t)
			if err != nil {
				t.Errorf("Scan failed: %v", err)
				// Clean up tracking on error
				s.scanningTasks.Delete(identity)
				return err
			}
			
			// Store in cache
			if s.cache != nil {
				if err := s.cache.Set(dep); err != nil {
					t.Warnf("Failed to cache scan results: %v", err)
				} else {
					t.Infof("Cached scan results")
				}
			}
			
			// Store result in task
			t.SetResult(dep)
			
			// Clean up tracking
			s.scanningTasks.Delete(identity)
			
			t.Success()
			return nil
		}),
	)
	
	// Track the task (may be a duplicate returned by manager)
	s.scanningTasks.Store(identity, scanTask)
	
	return scanTask
}

// performScan does the actual dependency scanning
func (s *DependencyScanner) performScan(ctx flanksourceContext.Context, gitURL, tag string, t *task.Task) (*Dependency, error) {
	// Simulate scanning process
	// In a real implementation, this would:
	// 1. Clone or fetch the repository
	// 2. Checkout the specific tag/hash
	// 3. Parse go.mod, package.json, requirements.txt, etc.
	// 4. Extract dependency information
	
	t.Infof("Cloning repository %s", gitURL)
	time.Sleep(500 * time.Millisecond) // Simulate clone
	
	t.Infof("Checking out tag %s", tag)
	time.Sleep(200 * time.Millisecond) // Simulate checkout
	
	t.Infof("Analyzing dependencies")
	time.Sleep(300 * time.Millisecond) // Simulate analysis
	
	// Return mock data for now
	return &Dependency{
		GitURL: gitURL,
		Tag:    tag,
		Hash:   "abc123def456", // Would be actual git hash
		Modules: []Module{
			{
				Name:    "main",
				Version: tag,
				Dependencies: []string{
					"github.com/spf13/cobra@v1.7.0",
					"github.com/spf13/viper@v1.16.0",
				},
			},
		},
		ScannedAt: time.Now(),
	}, nil
}

// createCompletedTask creates a task that is already completed with cached data
func (s *DependencyScanner) createCompletedTask(dep *Dependency) *task.Task {
	identity := fmt.Sprintf("dep-scan:%s@%s", dep.GitURL, dep.Tag)
	
	// Create a task that immediately completes with cached data
	cachedTask := s.manager.Start(
		fmt.Sprintf("Scanning %s@%s (cached)", dep.GitURL, dep.Tag),
		task.WithIdentity(identity),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Using cached scan results")
			t.SetResult(dep)
			t.Success()
			return nil
		}),
	)
	
	return cachedTask
}

// GetResult retrieves the scan result from a task
func (s *DependencyScanner) GetResult(scanTask *task.Task) (*Dependency, error) {
	result, err := scanTask.GetResult()
	if err != nil {
		return nil, err
	}
	
	if result == nil {
		return nil, fmt.Errorf("no scan result available")
	}
	
	dep, ok := result.(*Dependency)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	
	return dep, nil
}

// ClearCache clears all cached scan results
func (s *DependencyScanner) ClearCache() error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Clear()
}