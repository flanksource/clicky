package cache

// embeddedSchema contains the SQLite database schema
const embeddedSchema = `
-- AI Response Cache Database Schema

-- Main cache table for AI responses
CREATE TABLE IF NOT EXISTS ai_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_key TEXT NOT NULL,
    prompt_hash TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    error TEXT,
    
    -- Metrics
    tokens_input INTEGER DEFAULT 0,
    tokens_output INTEGER DEFAULT 0,
    tokens_cache_read INTEGER DEFAULT 0,
    tokens_cache_write INTEGER DEFAULT 0,
    tokens_total INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0.0,
    duration_ms INTEGER DEFAULT 0,
    
    -- Metadata
    project_name TEXT,
    task_name TEXT,
    temperature REAL DEFAULT 0.2,
    max_tokens INTEGER,
    session_id TEXT,
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    
    -- Indexes for efficient lookup
    UNIQUE(cache_key, model)
);

-- Index for cache lookups
CREATE INDEX IF NOT EXISTS idx_cache_lookup ON ai_cache(cache_key, model, expires_at);
CREATE INDEX IF NOT EXISTS idx_prompt_hash ON ai_cache(prompt_hash);
CREATE INDEX IF NOT EXISTS idx_created_at ON ai_cache(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_project ON ai_cache(project_name);
CREATE INDEX IF NOT EXISTS idx_model ON ai_cache(model);

-- Statistics table for aggregated metrics
CREATE TABLE IF NOT EXISTS ai_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date DATE NOT NULL,
    model TEXT NOT NULL,
    project_name TEXT,
    
    -- Daily aggregates
    request_count INTEGER DEFAULT 0,
    cache_hit_count INTEGER DEFAULT 0,
    cache_miss_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    
    -- Token metrics
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,
    total_cache_read_tokens INTEGER DEFAULT 0,
    total_cache_write_tokens INTEGER DEFAULT 0,
    
    -- Cost and performance
    total_cost_usd REAL DEFAULT 0.0,
    avg_duration_ms INTEGER DEFAULT 0,
    
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(date, model, project_name)
);

-- Session tracking for grouping related requests
CREATE TABLE IF NOT EXISTS ai_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL UNIQUE,
    command TEXT,
    args TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    total_requests INTEGER DEFAULT 0,
    total_cost_usd REAL DEFAULT 0.0,
    total_tokens INTEGER DEFAULT 0
);
`