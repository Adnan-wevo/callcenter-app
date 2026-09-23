/** Mirrors internal/softphone.Bootstrap on the Go side (softphone.go). */
export interface SoftphoneExtension {
  extension: string;
  display_name: string;
  is_default: boolean;
  is_supervisor: boolean;
  queues: string[];
}

export interface SupervisorCodes {
  spy_monitor: string;
  spy_whisper: string;
  spy_barge: string;
}

export interface SoftphoneBootstrap {
  server: string;
  port: number;
  ws_path: string;
  transport: string;

  extension: string;
  display_name: string;
  password: string;
  queues: string[];

  is_supervisor: boolean;
  supervisor: SupervisorCodes;
  extensions: SoftphoneExtension[];
}

// ---------------------------------------------------------------------------
// Live PBX snapshots — mirrors internal/gateway/pbxcontrol.Queue/Agent/etc.
// FlexString fields decode as plain strings on the Go side, so they arrive
// here as ordinary JSON strings too.
// ---------------------------------------------------------------------------

export interface LiveQueueCaller {
  caller_id: string;
  position: number;
  wait_seconds: number;
  unique_id: string;
  /** The Asterisk channel name — what queue/pickup and queue/redirect
   * actually take, NOT unique_id (see pbxcontrol.QueueCaller's own doc
   * comment on the Go side). */
  channel: string;
}

export interface LiveQueueMember {
  interface: string;
  name: string;
  status_code: number;
  status_text: string;
  paused: boolean;
  pause_reason: string;
  calls_taken_today: number;
  last_call_epoch: number;
}

export interface LiveQueue {
  name: string;
  strategy: string;
  calls_waiting: number;
  completed_today: number;
  abandoned_today: number;
  sla_percent: number | null;
  avg_hold_seconds: number | null;
  max_hold_seconds: number | null;
  avg_talk_seconds: number;
  members: LiveQueueMember[];
  callers: LiveQueueCaller[];
}

export interface LiveQueuesSnapshot {
  queues: Record<string, LiveQueue>;
}

/**
 * Every live-snapshot endpoint (`/softphone/queues`, `/softphone/agents`,
 * `/softphone/live-calls`) is built by handlers.liveSnapshot, which calls
 * `apires.Item(c, status, gin.H{"data": data, "meta": {...}})` — i.e. it
 * passes a `{data, meta}` object AS THE ENVELOPE'S `data` FIELD, not as the
 * envelope itself. ApiService.get() already unwraps the outer envelope
 * (`{status, data}` -> `data`), so what actually lands here is a SECOND
 * `{data, meta}` layer underneath that — the real snapshot is at
 * `.data`, not at the top level. Skipping this type and reading
 * `LiveQueuesSnapshot` straight off `ApiService.get()` is exactly what
 * crashed `queueWaitingCalls` with "Cannot convert undefined or null to
 * object": `snapshot.queues` was undefined because `snapshot` was actually
 * `{data: {queues: ...}, meta: ...}`.
 */
export interface LiveSnapshotEnvelope<T> {
  data: T;
  meta: {
    stale: boolean;
    age_seconds: number;
    generated_at: string;
  };
}

export interface LiveAgentQueueDetail {
  paused: boolean;
  pause_reason: string;
}

export interface LiveAgentActiveCall {
  unique_id: string;
  caller_id: string;
  queue: string;
}

export interface LiveAgent {
  extension: string;
  status: string;
  paused: boolean;
  pause_reason: string;
  queues: number[];
  queue_details: Record<string, LiveAgentQueueDetail>;
  calls_taken_today: number;
  active_call: LiveAgentActiveCall | null;
  last_event_epoch: number;
}

export interface LiveAgentsSnapshot {
  agents: Record<string, LiveAgent>;
}

/** One row in the softphone panel's Queue tab — a caller waiting, flattened
 * with the queue it is waiting in so calls from several queues can share
 * one list. */
export interface QueueWaitingCall extends LiveQueueCaller {
  queue: string;
}

/** One row in the Contacts tab — a coworker's extension, from the live
 * agents snapshot. */
export interface ContactRow {
  extension: string;
  status: string;
  paused: boolean;
  onCall: boolean;
}

/** Mirrors handlers.callLogEntry — the softphone panel's own History tab. */
export interface CallLogEntry {
  id: string;
  direction: 'inbound' | 'outbound' | string;
  status: string;
  caller_id: string;
  caller_name: string;
  destination: string;
  queue: string;
  wait_seconds: number;
  talk_seconds: number;
  started_at: string | null;
  answered_at: string | null;
  ended_at: string | null;
  has_recording: boolean;
}

export type DebugLevel = 'info' | 'warn' | 'error';

/** One entry in the client-only Logs tab — never sent to the server, purely
 * a running record of what THIS session's softphone did, matching
 * heal-crm's own client-side debug log (Alpine's `debugLogs`, never
 * persisted server-side there either). */
export interface DebugLogEntry {
  id: number;
  at: Date;
  level: DebugLevel;
  message: string;
}
