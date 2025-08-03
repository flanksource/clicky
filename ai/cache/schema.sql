-- AI Response Cache Database Schema
-- Location: ~/.cache/clicky-ai.db

-- Main cache table for AI responses
CREATE TABLE IF NOT EXISTS ai_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_key TEXT NOT NULL,  -- Hash of prompt + model + other params
    prompt_hash TEXT NOT NULL, -- Hash of just the prompt for history lookup
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
    project_name TEXT,          -- Project/repository name for filtering
    task_name TEXT,              -- Task identifier (e.g., commit hash, dependency name)
    temperature REAL DEFAULT 0.2,
    max_tokens INTEGER,
    
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

-- Link cache entries to sessions
ALTER TABLE ai_cache ADD COLUMN session_id TEXT REFERENCES ai_sessions(session_id);

-- View for history command
CREATE VIEW IF NOT EXISTS ai_history_view AS
SELECT 
    ac.id,
    ac.model,
    ac.task_name,
    ac.project_name,
    SUBSTR(ac.prompt, 1, 100) as prompt_preview,
    SUBSTR(ac.response, 1, 100) as response_preview,
    ac.tokens_total,
    ac.cost_usd,
    ac.duration_ms,
    ac.created_at,
    CASE 
        WHEN ac.error IS NOT NULL THEN 'error'
        WHEN ac.expires_at < CURRENT_TIMESTAMP THEN 'expired'
        ELSE 'valid'
    END as status
FROM ai_cache ac
ORDER BY ac.created_at DESC;

-- View for stats command
CREATE VIEW IF NOT EXISTS ai_stats_view AS
SELECT 
    model,
    project_name,
    COUNT(*) as total_requests,
    SUM(CASE WHEN error IS NULL THEN 1 ELSE 0 END) as successful_requests,
    SUM(CASE WHEN error IS NOT NULL THEN 1 ELSE 0 END) as failed_requests,
    SUM(tokens_total) as total_tokens,
    SUM(cost_usd) as total_cost,
    AVG(duration_ms) as avg_duration_ms,
    MIN(created_at) as first_request,
    MAX(created_at) as last_request
FROM ai_cache
GROUP BY model, project_name;