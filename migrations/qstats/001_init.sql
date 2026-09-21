-- Local-testing-only schema for the "qstats" database.
--
-- ASSUMPTION: every table/column here is a guess — see
-- internal/db/qstats/models.go for the full list of assumptions. This file
-- exists purely so docker-compose can boot a MySQL instance with something
-- to query against; REPLACE it with the real schema (or a dump/migration
-- derived from it) once the Laravel/pbx-worker reference lands.

CREATE TABLE IF NOT EXISTS queue_names (
    id INT AUTO_INCREMENT PRIMARY KEY,
    extension VARCHAR(32) NOT NULL,
    name VARCHAR(128) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS queue_agents (
    id INT AUTO_INCREMENT PRIMARY KEY,
    agent_id VARCHAR(32) NOT NULL,
    name VARCHAR(128) NOT NULL,
    queue_id INT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_queue_agents_queue_id (queue_id)
);

CREATE TABLE IF NOT EXISTS queue_stats (
    id INT AUTO_INCREMENT PRIMARY KEY,
    queue_id INT NOT NULL,
    agent_id VARCHAR(32) NULL,
    call_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    wait_seconds INT NOT NULL DEFAULT 0,
    talk_seconds INT NOT NULL DEFAULT 0,
    started_at DATETIME NOT NULL,
    ended_at DATETIME NULL,
    KEY idx_queue_stats_queue_id (queue_id),
    KEY idx_queue_stats_call_id (call_id)
);

CREATE TABLE IF NOT EXISTS queue_stats_mv (
    queue_id INT NOT NULL,
    stat_date DATE NOT NULL,
    total_calls INT NOT NULL DEFAULT 0,
    answered_calls INT NOT NULL DEFAULT 0,
    abandoned_calls INT NOT NULL DEFAULT 0,
    avg_wait_seconds DECIMAL(10,2) NOT NULL DEFAULT 0,
    avg_talk_seconds DECIMAL(10,2) NOT NULL DEFAULT 0,
    PRIMARY KEY (queue_id, stat_date)
);

CREATE TABLE IF NOT EXISTS queue_events (
    id INT AUTO_INCREMENT PRIMARY KEY,
    queue_id INT NOT NULL,
    agent_id VARCHAR(32) NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    event_time DATETIME NOT NULL,
    metadata VARCHAR(512) NULL,
    KEY idx_queue_events_queue_id (queue_id)
);

CREATE TABLE IF NOT EXISTS recordings (
    id INT AUTO_INCREMENT PRIMARY KEY,
    call_id VARCHAR(64) NOT NULL,
    queue_id INT NOT NULL,
    agent_id VARCHAR(32) NULL,
    file_path VARCHAR(512) NOT NULL,
    duration_seconds INT NOT NULL DEFAULT 0,
    recorded_at DATETIME NOT NULL,
    KEY idx_recordings_call_id (call_id)
);

-- Seed data so ListQueues / ListAgents return something on a fresh local boot.
INSERT INTO queue_names (id, extension, name) VALUES
    (1, '6001', 'Sales'),
    (2, '6002', 'Support')
ON DUPLICATE KEY UPDATE name = VALUES(name);

INSERT INTO queue_agents (id, agent_id, name, queue_id) VALUES
    (1, '101', 'Ahmad', 1),
    (2, '102', 'Siti', 2)
ON DUPLICATE KEY UPDATE name = VALUES(name);
