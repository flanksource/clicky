package cache

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	// ErrCacheDisabled indicates caching is disabled
	ErrCacheDisabled = errors.New("caching is disabled")
	// ErrNotFound indicates the entry was not found in cache
	ErrNotFound = errors.New("cache entry not found")
)

// Config holds cache configuration
type Config struct {
	DBPath  string        // Database file path (default: ~/.cache/clicky-ai.db)
	TTL     time.Duration // Cache time-to-live
	NoCache bool          // Disable caching
	Debug   bool          // Enable debug output
}

// Entry represents a cached AI response
type Entry struct {
	// Pointer (8 bytes)
	ExpiresAt *time.Time

	// Strings (16 bytes each on 64-bit)
	CacheKey    string
	PromptHash  string
	Model       string
	Prompt      string
	Response    string
	Error       string
	ProjectName string
	TaskName    string
	SessionID   string

	// 8-byte types
	ID          int64
	DurationMS  int64
	CostUSD     float64
	Temperature float64
	CreatedAt   time.Time
	AccessedAt  time.Time

	// 4-byte types
	TokensInput      int
	TokensOutput     int
	TokensCacheRead  int
	TokensCacheWrite int
	TokensTotal      int
	MaxTokens        int
}

// StatsEntry represents aggregated statistics
type StatsEntry struct {
	// Strings (16 bytes each)
	Model       string
	ProjectName string

	// 8-byte types
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalTokens        int64
	AvgDurationMS      int64
	TotalCost          float64
	FirstRequest       time.Time
	LastRequest        time.Time
}

// Cache manages AI response caching in SQLite
type Cache struct {
	db     *sql.DB
	config Config
}

// New creates a new cache instance
func New(config Config) (*Cache, error) {
	if config.DBPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.DBPath = filepath.Join(homeDir, ".cache", "clicky-ai.db")
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(config.DBPath)
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
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

	cache := &Cache{
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
func (c *Cache) Close() error {
	if err := c.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}

// generateCacheKey creates a unique key for cache lookup
func (c *Cache) generateCacheKey(prompt, model string, temperature float64, maxTokens int) string {
	data := fmt.Sprintf("%s|%s|%.2f|%d", prompt, model, temperature, maxTokens)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// generatePromptHash creates a hash of just the prompt for history lookup
func (c *Cache) generatePromptHash(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", hash)[:16] // Use first 16 chars for brevity
}

// Get retrieves a cached response
func (c *Cache) Get(prompt, model string, temperature float64, maxTokens int) (*Entry, error) {
	if c.config.NoCache {
		return nil, ErrCacheDisabled
	}

	cacheKey := c.generateCacheKey(prompt, model, temperature, maxTokens)

	query := `
		SELECT id, cache_key, prompt_hash, model, prompt, response, error,
		       tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, 
		       tokens_total, cost_usd, duration_ms, project_name, task_name,
		       temperature, max_tokens, created_at, accessed_at, expires_at, session_id
		FROM ai_cache
		WHERE cache_key = ? AND model = ? 
		  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
		ORDER BY created_at DESC
		LIMIT 1
	`

	var entry Entry
	var expiresAt sql.NullTime
	err := c.db.QueryRow(query, cacheKey, model).Scan(
		&entry.ID, &entry.CacheKey, &entry.PromptHash, &entry.Model,
		&entry.Prompt, &entry.Response, &entry.Error,
		&entry.TokensInput, &entry.TokensOutput, &entry.TokensCacheRead, &entry.TokensCacheWrite,
		&entry.TokensTotal, &entry.CostUSD, &entry.DurationMS,
		&entry.ProjectName, &entry.TaskName, &entry.Temperature, &entry.MaxTokens,
		&entry.CreatedAt, &entry.AccessedAt, &expiresAt, &entry.SessionID,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cache entry: %w", err)
	}

	if expiresAt.Valid {
		entry.ExpiresAt = &expiresAt.Time
	}

	// Update access time
	_, _ = c.db.Exec("UPDATE ai_cache SET accessed_at = CURRENT_TIMESTAMP WHERE id = ?", entry.ID)

	if c.config.Debug {
		fmt.Fprintf(os.Stderr, "Cache hit for prompt hash %s (model: %s)\n", entry.PromptHash, model)
	}

	return &entry, nil
}

// Set stores a response in the cache
func (c *Cache) Set(entry *Entry) error {
	if c.config.NoCache {
		return nil
	}

	// Generate keys
	entry.CacheKey = c.generateCacheKey(entry.Prompt, entry.Model, entry.Temperature, entry.MaxTokens)
	entry.PromptHash = c.generatePromptHash(entry.Prompt)

	// Calculate expiration
	var expiresAt *time.Time
	if c.config.TTL > 0 {
		exp := time.Now().Add(c.config.TTL)
		expiresAt = &exp
	}

	query := `
		INSERT OR REPLACE INTO ai_cache (
			cache_key, prompt_hash, model, prompt, response, error,
			tokens_input, tokens_output, tokens_cache_read, tokens_cache_write,
			tokens_total, cost_usd, duration_ms, project_name, task_name,
			temperature, max_tokens, expires_at, session_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := c.db.Exec(query,
		entry.CacheKey, entry.PromptHash, entry.Model, entry.Prompt, entry.Response, entry.Error,
		entry.TokensInput, entry.TokensOutput, entry.TokensCacheRead, entry.TokensCacheWrite,
		entry.TokensTotal, entry.CostUSD, entry.DurationMS,
		entry.ProjectName, entry.TaskName, entry.Temperature, entry.MaxTokens,
		expiresAt, entry.SessionID,
	)
	if err != nil {
		return fmt.Errorf("failed to set cache entry: %w", err)
	}

	if c.config.Debug {
		fmt.Fprintf(os.Stderr, "Cached response for prompt hash %s (model: %s, tokens: %d, cost: $%.6f)\n",
			entry.PromptHash, entry.Model, entry.TokensTotal, entry.CostUSD)
	}

	// Update daily stats
	c.updateStats(entry)

	return nil
}

// GetHistory retrieves recent AI interactions
func (c *Cache) GetHistory(limit int, projectName string) ([]Entry, error) {
	query := `
		SELECT id, cache_key, prompt_hash, model, prompt, response, error,
		       tokens_input, tokens_output, tokens_cache_read, tokens_cache_write,
		       tokens_total, cost_usd, duration_ms, project_name, task_name,
		       temperature, max_tokens, created_at, accessed_at, expires_at, session_id
		FROM ai_cache
		WHERE 1=1
	`

	args := []interface{}{}
	if projectName != "" {
		query += " AND project_name = ?"
		args = append(args, projectName)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		var expiresAt sql.NullTime
		err := rows.Scan(
			&entry.ID, &entry.CacheKey, &entry.PromptHash, &entry.Model,
			&entry.Prompt, &entry.Response, &entry.Error,
			&entry.TokensInput, &entry.TokensOutput, &entry.TokensCacheRead, &entry.TokensCacheWrite,
			&entry.TokensTotal, &entry.CostUSD, &entry.DurationMS,
			&entry.ProjectName, &entry.TaskName, &entry.Temperature, &entry.MaxTokens,
			&entry.CreatedAt, &entry.AccessedAt, &expiresAt, &entry.SessionID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		if expiresAt.Valid {
			entry.ExpiresAt = &expiresAt.Time
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetStats retrieves aggregated statistics
func (c *Cache) GetStats(projectName string) ([]StatsEntry, error) {
	query := `
		SELECT model, project_name,
		       COUNT(*) as total_requests,
		       SUM(CASE WHEN error IS NULL OR error = '' THEN 1 ELSE 0 END) as successful_requests,
		       SUM(CASE WHEN error IS NOT NULL AND error != '' THEN 1 ELSE 0 END) as failed_requests,
		       SUM(tokens_total) as total_tokens,
		       SUM(cost_usd) as total_cost,
		       AVG(duration_ms) as avg_duration_ms,
		       MIN(created_at) as first_request,
		       MAX(created_at) as last_request
		FROM ai_cache
		WHERE 1=1
	`

	args := []interface{}{}
	if projectName != "" {
		query += " AND project_name = ?"
		args = append(args, projectName)
	}

	query += " GROUP BY model, project_name ORDER BY total_requests DESC"

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	defer rows.Close()

	var stats []StatsEntry
	for rows.Next() {
		var s StatsEntry
		var projectNameNull sql.NullString
		err := rows.Scan(
			&s.Model, &projectNameNull,
			&s.TotalRequests, &s.SuccessfulRequests, &s.FailedRequests,
			&s.TotalTokens, &s.TotalCost, &s.AvgDurationMS,
			&s.FirstRequest, &s.LastRequest,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}
		if projectNameNull.Valid {
			s.ProjectName = projectNameNull.String
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// Clear removes all cache entries
func (c *Cache) Clear(projectName string) error {
	query := "DELETE FROM ai_cache WHERE 1=1"
	args := []interface{}{}

	if projectName != "" {
		query += " AND project_name = ?"
		args = append(args, projectName)
	}

	result, err := c.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	rows, _ := result.RowsAffected()
	if c.config.Debug {
		fmt.Fprintf(os.Stderr, "Cleared %d cache entries\n", rows)
	}

	return nil
}

// cleanupExpired removes expired cache entries
func (c *Cache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		query := "DELETE FROM ai_cache WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP"
		result, err := c.db.Exec(query)
		if err != nil {
			if c.config.Debug {
				fmt.Fprintf(os.Stderr, "Failed to cleanup expired entries: %v\n", err)
			}
			continue
		}

		if rows, _ := result.RowsAffected(); rows > 0 && c.config.Debug {
			fmt.Fprintf(os.Stderr, "Cleaned up %d expired cache entries\n", rows)
		}
	}
}

// updateStats updates daily statistics
func (c *Cache) updateStats(entry *Entry) {
	date := entry.CreatedAt.Format("2006-01-02")

	query := `
		INSERT INTO ai_stats (
			date, model, project_name, request_count, 
			total_input_tokens, total_output_tokens, 
			total_cache_read_tokens, total_cache_write_tokens,
			total_cost_usd
		) VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?)
		ON CONFLICT(date, model, project_name) DO UPDATE SET
			request_count = request_count + 1,
			total_input_tokens = total_input_tokens + excluded.total_input_tokens,
			total_output_tokens = total_output_tokens + excluded.total_output_tokens,
			total_cache_read_tokens = total_cache_read_tokens + excluded.total_cache_read_tokens,
			total_cache_write_tokens = total_cache_write_tokens + excluded.total_cache_write_tokens,
			total_cost_usd = total_cost_usd + excluded.total_cost_usd,
			updated_at = CURRENT_TIMESTAMP
	`

	_, _ = c.db.Exec(query,
		date, entry.Model, entry.ProjectName,
		entry.TokensInput, entry.TokensOutput,
		entry.TokensCacheRead, entry.TokensCacheWrite,
		entry.CostUSD,
	)
}

// initSchema creates the database schema
func (c *Cache) initSchema() error {
	schemaPath := filepath.Join(filepath.Dir(c.config.DBPath), "schema.sql")

	// Try to read embedded schema first, fall back to file
	schema := embeddedSchema
	if _, err := os.Stat(schemaPath); err == nil {
		if data, err := os.ReadFile(schemaPath); err == nil {
			schema = string(data)
		}
	}

	if _, err := c.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

// ExportToJSON exports cache entries to JSON
func (c *Cache) ExportToJSON(w io.Writer, projectName string) error {
	entries, err := c.GetHistory(0, projectName) // 0 means no limit
	if err != nil {
		return fmt.Errorf("failed to get entries: %w", err)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

// ImportFromJSON imports cache entries from JSON
func (c *Cache) ImportFromJSON(r io.Reader) error {
	var entries []Entry
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&entries); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	for _, entry := range entries {
		if err := c.Set(&entry); err != nil {
			return fmt.Errorf("failed to import entry: %w", err)
		}
	}

	return nil
}
