-- Mirrors Modules/SoftPhone/database/migrations/2026_04_17_000001_create_sip_extensions_table.php
-- and its 2026_04_17_000005 cid_names follow-up, column for column.
--
-- IMPORTANT — a real discrepancy in heal-crm itself, carried forward
-- deliberately: SipSwitchController.php's own doc comment says "Allows a
-- user with multiple SIP extensions to switch the active account", and
-- Softphone::with() queries SipExtension::where('user_id', ...)->get() (a
-- collection, implying several rows are expected). But the migration puts
-- UNIQUE on user_id — the database allows exactly ONE extension per user
-- no matter what the application code above it assumes. The constraint is
-- what actually governs production behaviour, so it is what this schema
-- enforces too; the Go code that reads it (internal/softphone) is written
-- against "zero or one extension per user", not "a list", even though a
-- couple of call sites still shape their response as a single-element
-- array for API compatibility with a client written against the aspirational
-- multi-extension design.
USE callcenter;

CREATE TABLE IF NOT EXISTS sip_extensions (
    id CHAR(36) NOT NULL PRIMARY KEY,
    user_id CHAR(36) NOT NULL UNIQUE,
    extension VARCHAR(20) NOT NULL UNIQUE,
    sip_username VARCHAR(120) NOT NULL,
    -- AES-256-GCM ciphertext, base64: nonce||ciphertext||tag. See
    -- internal/softphone/crypto.go. Laravel encrypts this with Crypt
    -- (AES-256-CBC + app key via its own envelope); this is not
    -- byte-compatible with that envelope, and does not need to be — the two
    -- systems each decrypt only what they themselves encrypted, this is a
    -- fresh product database, not a shared one.
    sip_password_encrypted TEXT NOT NULL,
    display_name VARCHAR(120) NULL,
    cid_names JSON NULL,
    queues JSON NULL,
    is_supervisor TINYINT(1) NOT NULL DEFAULT 0,
    is_default TINYINT(1) NOT NULL DEFAULT 0,
    last_registered_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_sip_extensions_extension (extension),
    KEY idx_sip_extensions_display_name (display_name)
);
