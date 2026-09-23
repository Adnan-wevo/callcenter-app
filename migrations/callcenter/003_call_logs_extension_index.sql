-- Supports the softphone panel's own "History" tab: an agent's own past
-- calls, looked up by their extension and ordered newest-first. None of
-- call_logs' existing keys lead with extension (they lead with unique_id,
-- channel, caller_id or destination — all built for detectQueue()'s
-- reconciliation lookups, not for "show me my calls"), so that query would
-- otherwise scan.
USE callcenter;

ALTER TABLE call_logs
    ADD KEY idx_call_logs_extension_started (extension, started_at);
