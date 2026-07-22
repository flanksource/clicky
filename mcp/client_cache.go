package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CachedTool is the durable subset of a tool definition needed to construct
// an offline CLI command.
type CachedTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// CatalogCache records a server catalog and the conditions under which it was
// fetched. Transport stores the winning protocol when the config uses auto.
type CatalogCache struct {
	Fingerprint   string        `json:"fingerprint"`
	FetchedAt     time.Time     `json:"fetchedAt"`
	TTL           time.Duration `json:"ttl"`
	Transport     string        `json:"transport"`
	ServerName    string        `json:"serverName,omitempty"`
	ServerVersion string        `json:"serverVersion,omitempty"`
	Tools         []CachedTool  `json:"tools"`
}

// Stale reports whether the catalog was fetched for different behavior or is
// older than its configured lifetime.
func (c *CatalogCache) Stale(cfg ServerConfig, now time.Time) bool {
	if c == nil || c.Fingerprint != cfg.Fingerprint() {
		return true
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = cfg.cacheTTL()
	}
	return c.FetchedAt.IsZero() || !now.Before(c.FetchedAt.Add(ttl))
}

func catalogPath(registry *ServerRegistry, name string) string {
	return filepath.Join(registry.cacheDir(), name+".json")
}

// LoadCatalog reads a cached catalog. Corruption is self-healed as a miss so a
// broken cache cannot permanently disable CLI help.
func LoadCatalog(registry *ServerRegistry, name string) (*CatalogCache, error) {
	path := catalogPath(registry, name)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read catalog cache: %w", err)
	}
	var catalog CatalogCache
	if err := json.Unmarshal(data, &catalog); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, fmt.Errorf("discard corrupt catalog cache: %w", removeErr)
		}
		return nil, nil
	}
	return &catalog, nil
}

// SaveCatalog atomically stores a catalog with private permissions.
func SaveCatalog(registry *ServerRegistry, name string, catalog *CatalogCache) error {
	if catalog == nil {
		return fmt.Errorf("cannot save a nil catalog")
	}
	return writeJSONAtomic(catalogPath(registry, name), catalog)
}
