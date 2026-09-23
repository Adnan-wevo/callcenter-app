-- Mirrors Modules/CallCenter/database/migrations/..._create_call_center_queue_groups_table.php
-- and its _queues sibling, minus SoftDeletes (this service hard-deletes,
-- matching sip_extensions/call_center_settings above) and minus the
-- RecordLock-backed "being edited by X" concurrent-edit warning (no
-- presence system in this service to back it).
--
-- Organisational metadata only today — nothing in this Go service's
-- reports groups BY queue group yet (heal-crm's own report screens don't
-- either, from what this repo could confirm), so this is a management CRUD
-- with no behavioural consumer, same honest scope note as
-- 004_call_center_settings.sql's skipped categories.
USE callcenter;

CREATE TABLE IF NOT EXISTS call_center_queue_groups (
    id CHAR(36) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1000) NULL,
    is_active TINYINT(1) NOT NULL DEFAULT 1,
    sort_order INT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_queue_groups_name (name)
);

CREATE TABLE IF NOT EXISTS call_center_queue_group_queues (
    id CHAR(36) NOT NULL PRIMARY KEY,
    queue_group_id CHAR(36) NOT NULL,
    queue_name VARCHAR(50) NOT NULL,

    KEY idx_qgq_group (queue_group_id),
    CONSTRAINT fk_qgq_group FOREIGN KEY (queue_group_id)
        REFERENCES call_center_queue_groups (id) ON DELETE CASCADE
);
