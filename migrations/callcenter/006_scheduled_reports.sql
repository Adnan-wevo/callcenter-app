-- Mirrors Modules/CallCenter/app/Models/ScheduledReport.php's fillable
-- columns. Storage only — there is no email sender and no cron/scheduler
-- process anywhere in this Go service to actually EXECUTE these rows yet
-- (confirmed by search: no smtp/net-mail usage, no ticker-driven job
-- runner). A row here is a saved intent, not a running schedule; the admin
-- screen built on this table says so explicitly rather than implying
-- delivery that doesn't happen.
USE callcenter;

CREATE TABLE IF NOT EXISTS call_center_scheduled_reports (
    id CHAR(36) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    destination_email VARCHAR(255) NOT NULL,
    reports JSON NOT NULL,
    queues JSON NULL,
    last_days INT NOT NULL DEFAULT 1,
    cron_day_month VARCHAR(20) NOT NULL DEFAULT '*',
    cron_day_week VARCHAR(20) NOT NULL DEFAULT '*',
    cron_hour VARCHAR(5) NOT NULL DEFAULT '8',
    cron_minute VARCHAR(5) NOT NULL DEFAULT '0',
    is_active TINYINT(1) NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_scheduled_reports_name (name)
);
