-- Local-testing-only schema for the "qstats" database.
--
-- Table names, primary keys and the columns below are taken from the REAL
-- Laravel Eloquent models (Modules/CallCenter/app/Models/*.php) — see
-- internal/db/qstats/models.go.
--
-- Two caveats:
--
--  1. The real qstats database has NO migrations anywhere in the Laravel app.
--     It is owned by the PBX server (config/tenancy.php says so). This file
--     is NOT that schema — it is the subset this service reads, shaped so
--     docker-compose has something to query against locally.
--  2. queue_stats and queue_stats_mv declare no $fillable in Laravel, so
--     their full column lists are unknown. Only the columns confirmed via the
--     models' relations and casts are here. Get the rest from a live
--     `SHOW CREATE TABLE` before relying on them.

-- Queue name lookup. Real table is `qname`, PK `queue_id`, label column
-- `queue` — NOT `queue_names`/`id`/`name`.
CREATE TABLE IF NOT EXISTS qname (
    queue_id INT AUTO_INCREMENT PRIMARY KEY,
    queue VARCHAR(128) NOT NULL
);

-- Agent name lookup. Real table is `qagent`, PK `agent_id`, label column
-- `agent`. Note there is NO queue linkage column: agents are not tied to a
-- single queue at this table's row level.
CREATE TABLE IF NOT EXISTS qagent (
    agent_id INT AUTO_INCREMENT PRIMARY KEY,
    agent VARCHAR(128) NOT NULL
);

-- Event-type lookup. A dimension table (id + label), NOT an event log.
CREATE TABLE IF NOT EXISTS qevent (
    event_id INT AUTO_INCREMENT PRIMARY KEY,
    event VARCHAR(64) NOT NULL
);

-- One row per call event. The FK column names are unusual and deliberate:
-- `qname` is the FK to qname.queue_id (same name as the table it points at),
-- likewise `qagent` and `qevent`. `uniqueid` is the Asterisk call id and the
-- join key to recordings.
CREATE TABLE IF NOT EXISTS queue_stats (
    queue_stats_id INT AUTO_INCREMENT PRIMARY KEY,
    datetime DATETIME NOT NULL,
    qname INT NOT NULL,
    qagent INT NULL,
    qevent INT NULL,
    uniqueid VARCHAR(64) NULL,
    KEY idx_queue_stats_qname (qname),
    KEY idx_queue_stats_uniqueid (uniqueid)
);

-- Only the three datetime columns the Eloquent model casts are confirmed.
-- Their presence (connect + end per row) indicates call-lifecycle
-- granularity, NOT a daily aggregate rollup.
CREATE TABLE IF NOT EXISTS queue_stats_mv (
    id INT AUTO_INCREMENT PRIMARY KEY,
    datetime DATETIME NOT NULL,
    datetimeconnect DATETIME NULL,
    datetimeend DATETIME NULL
);

-- PK is `uniqueid`: a STRING, not an auto-increment int.
CREATE TABLE IF NOT EXISTS recordings (
    uniqueid VARCHAR(64) NOT NULL PRIMARY KEY,
    filename VARCHAR(255) NOT NULL
);

INSERT INTO qname (queue_id, queue) VALUES
    (1, 'Sales'),
    (2, 'Support');

INSERT INTO qagent (agent_id, agent) VALUES
    (101, 'Ahmad'),
    (102, 'Siti');

INSERT INTO qevent (event_id, event) VALUES
    (1, 'COMPLETEAGENT'),
    (2, 'COMPLETECALLER'),
    (3, 'ABANDON'),
    (4, 'EXITWITHTIMEOUT');

INSERT INTO recordings (uniqueid, filename) VALUES
    ('1758358812.101', '1758358812.101.wav');

INSERT INTO queue_stats (datetime, qname, qagent, qevent, uniqueid) VALUES
    ('2026-09-20 09:00:12', 1, 101, 1, '1758358812.101'),
    ('2026-09-20 10:01:30', 2, NULL, 3, '1758362490.102');
