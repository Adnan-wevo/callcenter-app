-- The product's OWN operational database — separate from qstats (externally
-- owned by the PBX, see migrations/qstats/001_init.sql) and separate from
-- heal-crm's database (this product does not have one).
--
-- call_logs mirrors Modules/SoftPhone/app/Models/CallLog.php's fillable
-- columns exactly (heal-crm's real schema, read directly from the model),
-- since this table's purpose is byte-for-byte the same one: the browser
-- softphone's own record of a call, reconciled against whatever the PBX
-- worker's poller already wrote for the same call. See internal/calllog.
--
-- The docker-compose mysql container's docker-entrypoint-initdb.d runner
-- executes every mounted .sql file against MYSQL_DATABASE (qstats) by
-- default, so this file creates and switches into its OWN database
-- explicitly rather than landing its table inside qstats.
CREATE DATABASE IF NOT EXISTS callcenter;
USE callcenter;

CREATE TABLE IF NOT EXISTS call_logs (
    id CHAR(36) NOT NULL PRIMARY KEY,
    user_id CHAR(36) NULL,
    extension VARCHAR(20) NULL,
    call_id VARCHAR(64) NULL,
    unique_id VARCHAR(64) NULL,
    channel VARCHAR(128) NULL,
    direction VARCHAR(10) NOT NULL,
    caller_id VARCHAR(32) NULL,
    caller_name VARCHAR(128) NULL,
    destination VARCHAR(32) NULL,
    queue VARCHAR(50) NULL,
    status VARCHAR(20) NOT NULL,
    wait_seconds INT NOT NULL DEFAULT 0,
    talk_seconds INT NOT NULL DEFAULT 0,
    started_at DATETIME NULL,
    answered_at DATETIME NULL,
    ended_at DATETIME NULL,
    metadata JSON NULL,
    recording_file VARCHAR(255) NULL,
    recording_format VARCHAR(10) NULL,
    recording_fetched_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- Match priority in detectQueue()/createCallLog(): unique_id first
    -- (strongest), then channel within a short window, then caller_id+queue
    -- within a short window. All three need to be fast lookups.
    KEY idx_call_logs_unique_id (unique_id),
    KEY idx_call_logs_channel (channel, started_at),
    KEY idx_call_logs_caller_started (caller_id, started_at),
    KEY idx_call_logs_status_caller_started (status, caller_id, started_at),
    KEY idx_call_logs_destination_started (destination, started_at)
);
