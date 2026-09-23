-- Resolves open decision D3 (internal/reports/settings.go's own doc
-- comment): "this service cannot read that table yet". This table is that
-- table — same category/key/value shape as heal-crm's own
-- call_center_settings, seeded with only the keys this Go service actually
-- CONSUMES (reports.Defaults()'s three fields). heal-crm also has
-- time_format/display/realtime categories, but nothing in this service
-- reads those yet, so they are deliberately not seeded here — a settings
-- screen offering a control with no effect is worse than one screen
-- narrower than heal-crm's.
USE callcenter;

CREATE TABLE IF NOT EXISTS call_center_settings (
    id CHAR(36) NOT NULL PRIMARY KEY,
    category VARCHAR(50) NOT NULL,
    setting_key VARCHAR(100) NOT NULL,
    value VARCHAR(255) NULL,
    description VARCHAR(255) NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uniq_category_key (category, setting_key)
);

INSERT IGNORE INTO call_center_settings (id, category, setting_key, value, description) VALUES
    (UUID(), 'sla', 'sla_interval', '20', 'SLA target time in seconds - calls answered within this meet SLA'),
    (UUID(), 'threshold', 'short_abandon_threshold', '5', 'Calls abandoned within this many seconds are dropped from every unanswered report entirely'),
    (UUID(), 'threshold', 'wrap_up', '0', 'Agent wrap-up time in seconds after call end, added to occupancy in Agent Performance');
