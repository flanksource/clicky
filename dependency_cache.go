package clicky

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// CacheConfig holds cache configuration
type CacheConfig struct {
	TTL    time.Duration // Cache time-to-live (0 means no caching)
	DBPath string        // Database file path
}

// DependencyCache manages caching of dependency scan results
type DependencyCache struct {
	db     *sql.DB
	config CacheConfig
}

// NewDependencyCache creates a new dependency cache
func NewDependencyCache(config CacheConfig) (*DependencyCache, error) {
	// If TTL is 0, caching is disabled
	if config.TTL == 0 {
		return &DependencyCache{
			config: config,
		}, nil
	}
	
	if config.DBPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.DBPath = filepath.Join(homeDir, ".cache", "clicky-deps.db")
	}
	
	// Ensure cache directory exists
	cacheDir := filepath.Dir(config.DBPath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	
	// Open database
	db, err := sql.Open("sqlite3", config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	// Set pragmas for performance
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -64000", // 64MB cache
		"PRAGMA busy_timeout = 5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma %s: %w", pragma, err)
		}
	}
	
	cache := &DependencyCache{
		db:     db,
		config: config,
	}
	
	// Initialize schema
	if err := cache.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	
	// Clean expired entries periodically
	go cache.cleanupExpired()
	
	return cache, nil
}

// Close closes the database connection
func (c *DependencyCache) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

// generateCacheKey creates a unique key for cache lookup
func (c *DependencyCache) generateCacheKey(gitURL, tag string) string {
	data := fmt.Sprintf("%s@%s", gitURL, tag)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// Get retrieves a cached dependency scan result
func (c *DependencyCache) Get(gitURL, tag string) (*Dependency, error) {
	// If caching is disabled (TTL = 0), always return nil
	if c.config.TTL == 0 || c.db == nil {
		return nil, nil
	}
	
	cacheKey := c.generateCacheKey(gitURL, tag)
	
	query := `
		SELECT git_url, tag, hash, modules_json, scanned_at, cached_at
		FROM dependency_cache
		WHERE cache_key = ? 
		  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
		ORDER BY cached_at DESC
		LIMIT 1
	`
	
	var dep Dependency
	var modulesJSON string
	var cachedAt time.Time
	
	err := c.db.QueryRow(query, cacheKey).Scan(
		&dep.GitURL, &dep.Tag, &dep.Hash,
		&modulesJSON, &dep.ScannedAt, &cachedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cache entry: %w", err)
	}
	
	// Parse modules JSON
	if err := json.Unmarshal([]byte(modulesJSON), &dep.Modules); err != nil {
		return nil, fmt.Errorf("failed to parse modules JSON: %w", err)
	}
	
	dep.CachedAt = &cachedAt
	
	// Update access time
	_, _ = c.db.Exec("UPDATE dependency_cache SET accessed_at = CURRENT_TIMESTAMP WHERE cache_key = ?", cacheKey)
	
	return &dep, nil
}

// Set stores a dependency scan result in the cache
func (c *DependencyCache) Set(dep *Dependency) error {
	// If caching is disabled (TTL = 0), don't store
	if c.config.TTL == 0 || c.db == nil {
		return nil
	}
	
	cacheKey := c.generateCacheKey(dep.GitURL, dep.Tag)
	
	// Serialize modules to JSON
	modulesJSON, err := json.Marshal(dep.Modules)
	if err != nil {
		return fmt.Errorf("failed to marshal modules: %w", err)
	}
	
	// Calculate expiration
	expiresAt := time.Now().Add(c.config.TTL)
	
	query := `
		INSERT OR REPLACE INTO dependency_cache (
			cache_key, git_url, tag, hash, modules_json,
			scanned_at, cached_at, expires_at, accessed_at
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP)
	`
	
	_, err = c.db.Exec(query,
		cacheKey, dep.GitURL, dep.Tag, dep.Hash,
		string(modulesJSON), dep.ScannedAt, expiresAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to set cache entry: %w", err)
	}
	
	return nil
}

// Clear removes all cache entries
func (c *DependencyCache) Clear() error {
	if c.db == nil {
		return nil
	}
	
	_, err := c.db.Exec("DELETE FROM dependency_cache")
	if err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	
	return nil
}

// cleanupExpired removes expired cache entries periodically
func (c *DependencyCache) cleanupExpired() {
	if c.db == nil {
		return
	}
	
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		query := "DELETE FROM dependency_cache WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP"
		_, _ = c.db.Exec(query)
	}
}

// initSchema creates the database schema
func (c *DependencyCache) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS dependency_cache (
		cache_key TEXT PRIMARY KEY,
		git_url TEXT NOT NULL,
		tag TEXT NOT NULL,
		hash TEXT,
		modules_json TEXT,
		scanned_at TIMESTAMP,
		cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP,
		accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_dependency_cache_expires 
		ON dependency_cache(expires_at);
	
	CREATE INDEX IF NOT EXISTS idx_dependency_cache_git_tag 
		ON dependency_cache(git_url, tag);
	`
	
	if _, err := c.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}
	
	return nil
}