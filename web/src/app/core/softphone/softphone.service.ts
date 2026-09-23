import { Injectable, computed, effect, inject, signal } from '@angular/core';
import { Subscription } from 'rxjs';

import { environment } from '../../environments/environment';
import { FakeSipEngine } from '../../engines/sip/test_fake/fake-sip-engine';
import {
  CallId,
  CallSession,
  DtmfDigit,
  RegistrationState,
  SipEngine,
} from '../../engines/sip/domain/sip-engine';
import { JsSipEngine } from '../../engines/sip/webrtc/jssip-engine';
import { ApiService } from '../api/api.service';
import { ApiMeta } from '../api/api.types';
import {
  CallLogEntry,
  ContactRow,
  DebugLevel,
  DebugLogEntry,
  LiveAgent,
  LiveAgentsSnapshot,
  LiveQueuesSnapshot,
  LiveSnapshotEnvelope,
  QueueWaitingCall,
  SoftphoneBootstrap,
} from './softphone.types';

export type ConnectState = 'idle' | 'connecting' | RegistrationState | 'no-extension' | 'error';

/** pbxcontrol.SpyMode's three values, as the client sends them. */
export type SpyMode = 'monitor' | 'whisper' | 'barge';

/** Per-call bookkeeping for the call-log write-path — see `_callMeta`. */
interface CallMeta {
  callLogId: string | null;
  queue: string | null;
  answeredSent: boolean;
  finalizedSent: boolean;
}

/**
 * Orchestrates the softphone for the whole app: fetches the caller's SIP
 * credentials, brings up a SipEngine, and exposes reactive state the panel
 * renders.
 *
 * `providedIn: 'root'` and instantiated once from the shell (see
 * `layout/callcenter-sidebar.ts`'s sibling, the softphone panel) — this is
 * the equivalent of heal-crm's `@persist('softphone-panel')`: the call must
 * survive the agent navigating between report screens, so it cannot live
 * inside a routed component that gets destroyed on navigation.
 *
 * Which engine backs it is an environment flag
 * (`environment.sipEngine`), not a runtime choice — swapping engines
 * mid-session would mean re-registering and would drop an active call.
 */
@Injectable({ providedIn: 'root' })
export class SoftphoneService {
  private readonly api = inject(ApiService);

  private engine: SipEngine | null = null;
  private subs: Subscription[] = [];

  private readonly _connectState = signal<ConnectState>('idle');
  private readonly _connectError = signal<string | null>(null);
  private readonly _bootstrap = signal<SoftphoneBootstrap | null>(null);
  private readonly _calls = signal<CallSession[]>([]);
  private readonly _remoteStream = signal<MediaStream | null>(null);

  // Queue/contacts snapshots — polled from pbx-worker while registered (see
  // startLivePolling below). Not sockets: this app has no realtime push
  // today, so a short poll is the whole mechanism, matching the plainest
  // reading of "and so on" in the Queue tab — good enough to see who's
  // waiting and who's on a call without heal-crm's Echo/Reverb machinery.
  private readonly _liveQueues = signal<LiveQueuesSnapshot | null>(null);
  private readonly _liveAgents = signal<LiveAgentsSnapshot | null>(null);
  private pollTimer: ReturnType<typeof setInterval> | null = null;

  private readonly _queueBusy = signal(false);
  private readonly _queueError = signal<string | null>(null);

  private readonly _historyEntries = signal<CallLogEntry[]>([]);
  private readonly _historyMeta = signal<ApiMeta | null>(null);

  private readonly _logs = signal<DebugLogEntry[]>([]);
  private logSeq = 0;

  readonly connectState = this._connectState.asReadonly();
  readonly connectError = this._connectError.asReadonly();
  readonly bootstrap = this._bootstrap.asReadonly();
  readonly calls = this._calls.asReadonly();
  /** The far end's audio, for the panel to attach to an <audio> element. */
  readonly remoteStream = this._remoteStream.asReadonly();

  readonly liveQueues = this._liveQueues.asReadonly();
  readonly liveAgents = this._liveAgents.asReadonly();
  readonly queueBusy = this._queueBusy.asReadonly();
  readonly queueError = this._queueError.asReadonly();
  readonly historyEntries = this._historyEntries.asReadonly();
  readonly historyMeta = this._historyMeta.asReadonly();
  /** Newest first, capped — see pushLog's own doc comment. */
  readonly logs = this._logs.asReadonly();

  readonly isConnected = computed(() => this._connectState() === 'registered');

  /** This agent's own row in the live agents snapshot — the authoritative
   * source for "am I logged into a queue right now / am I paused", the same
   * way heal-crm's `_queueSyncState()` treats AMI as authoritative over
   * whatever was cached client-side. */
  readonly selfAgent = computed<LiveAgent | null>(() => {
    const bootstrap = this._bootstrap();
    const agents = this._liveAgents();
    if (!bootstrap || !agents) {
      return null;
    }
    return agents.agents[bootstrap.extension] ?? null;
  });

  readonly queueLoggedIn = computed(() => (this.selfAgent()?.queues.length ?? 0) > 0);
  readonly queuePaused = computed(() => this.selfAgent()?.paused ?? false);

  /** Every caller currently waiting, across every queue, flattened with the
   * queue it's waiting in — the Queue tab's own list. */
  readonly queueWaitingCalls = computed<QueueWaitingCall[]>(() => {
    const snapshot = this._liveQueues();
    if (!snapshot) {
      return [];
    }
    const rows: QueueWaitingCall[] = [];
    for (const [queueId, queue] of Object.entries(snapshot.queues)) {
      for (const caller of queue.callers) {
        rows.push({ ...caller, queue: queue.name || queueId });
      }
    }
    return rows;
  });

  /** The queue ids currently on screen, for the "All Queues" filter. */
  readonly queueNames = computed<string[]>(() => {
    const snapshot = this._liveQueues();
    return snapshot ? Object.keys(snapshot.queues) : [];
  });

  /** The Contacts tab's own list: every extension the PBX currently knows
   * about, live. */
  readonly contacts = computed<ContactRow[]>(() => {
    const snapshot = this._liveAgents();
    if (!snapshot) {
      return [];
    }
    return Object.entries(snapshot.agents).map(([key, agent]) => ({
      extension: agent.extension || key,
      status: agent.status,
      paused: agent.paused,
      onCall: agent.active_call != null,
    }));
  });

  /** Coworkers currently on a call — who a supervisor may monitor/whisper/barge into. */
  readonly agentsOnCall = computed<LiveAgent[]>(() =>
    Object.values(this._liveAgents()?.agents ?? {}).filter((a) => a.active_call != null),
  );

  /** A call ringing inbound and not yet answered. At most one is shown. */
  readonly incomingCall = computed(() =>
    this._calls().find((c) => c.direction === 'inbound' && c.state === 'ringing'),
  );

  /** Every call that is neither the fresh incoming ring nor finished. */
  readonly activeCalls = computed(() =>
    this._calls().filter(
      (c) => c.state !== 'ended' && c.state !== 'failed' && c !== this.incomingCall(),
    ),
  );

  /**
   * The call the app treats as "the" call for the single-call surfaces —
   * the in-call card, the new call-detail panel, mute/hold/hangup — the
   * most recently changed active call. Concurrent calls beyond this one
   * (call waiting) are listed in `activeCalls` but not the primary focus;
   * multi-call juggling is out of scope. Lives here (not on the panel
   * component) because the new right-side call-detail panel needs the same
   * value the left softphone panel does.
   */
  readonly primaryCall = computed<CallSession | undefined>(() => {
    const calls = this.activeCalls();
    return calls[calls.length - 1];
  });

  /** `primaryCall`, narrowed to only once it's actually connected — the
   * call-detail panel's own trigger ("lepas pickup call"): it must not
   * show for an outbound call still ringing out, only once bridged. */
  readonly answeredCall = computed<CallSession | undefined>(() => {
    const call = this.primaryCall();
    return call && (call.state === 'answered' || call.state === 'held') ? call : undefined;
  });

  private readonly _now = signal(Date.now());
  private tickTimer: ReturnType<typeof setInterval> | null = null;

  /** `m:ss` since the primary call was answered, or null before that / with
   * no primary call. Ticks only while there is an answered call to time —
   * see the effect below. */
  readonly elapsed = computed(() => {
    const call = this.primaryCall();
    if (!call?.answeredAt) {
      return null;
    }
    const seconds = Math.max(0, Math.floor((this._now() - call.answeredAt.getTime()) / 1000));
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  });

  /** Per-call bookkeeping for the call-log write-path (§ below) — never
   * rendered directly; `primaryCallQueue` is the one derived read the UI
   * uses. Keyed by CallId so concurrent calls (call waiting) don't clobber
   * each other's state. */
  private readonly _callMeta = signal<Record<CallId, CallMeta>>({});

  /** The queue the primary call arrived through, once resolved — from
   * `detect-queue` if this app created the call-log row, falling back to
   * the live agent snapshot's own `active_call.queue` (already polled)
   * while that resolves. Raw queue id, not a friendly name: pbx-worker's
   * `queue-names` action returns friendly labels with no id to join them
   * against the live snapshot's numeric queue ids by, so there is no
   * "Sales"/"Support"-style label available for a LIVE call today — see
   * `ListQueues`' own doc comment in queues.go. */
  readonly primaryCallQueue = computed<string | null>(() => {
    const call = this.primaryCall();
    return call ? this.queueForCall(call) : null;
  });

  /** Resolved queue for any call, not just the primary one — the
   * incoming-call popup needs this for the still-ringing call, which
   * `primaryCall` deliberately excludes. Reads `_callMeta()`, so it stays
   * reactive when called from a template (see `primaryCallQueue`'s own
   * usage for the pattern). */
  queueForCall(call: CallSession): string | null {
    const resolved = this._callMeta()[call.id]?.queue;
    if (resolved) {
      return resolved;
    }
    const active = this.selfAgent()?.active_call;
    return active && active.caller_id === call.remote ? active.queue || null : null;
  }

  constructor() {
    // Runs the whole app's lifetime (this service is never destroyed), so
    // this only needs to start/stop the interval, never tear itself down.
    effect(() => {
      const answered = this.primaryCall()?.answeredAt != null;
      if (answered && !this.tickTimer) {
        this.tickTimer = setInterval(() => this._now.set(Date.now()), 1000);
      } else if (!answered && this.tickTimer) {
        clearInterval(this.tickTimer);
        this.tickTimer = null;
      }
    });
  }

  /**
   * Fetch this agent's credentials and register. Called once from the shell
   * after sign-in; a second call while already connecting/connected is a
   * no-op, since re-registering would drop whatever call is in progress.
   */
  connect(): void {
    if (this._connectState() !== 'idle' && this._connectState() !== 'error' && this._connectState() !== 'no-extension') {
      return;
    }

    this._connectState.set('connecting');
    this._connectError.set(null);

    this.api.get<SoftphoneBootstrap>('secure/softphone/bootstrap', undefined, { silent: true }).subscribe({
      next: (bootstrap) => {
        this._bootstrap.set(bootstrap);
        void this.registerWith(bootstrap);
      },
      error: (err: unknown) => {
        const status = (err as { status?: number })?.status;
        if (status === 404) {
          // Not every account has a phone — a report-only supervisor, say.
          // This is a normal outcome, not a failure to surface as an error.
          this._connectState.set('no-extension');
          return;
        }
        this._connectState.set('error');
        this._connectError.set('Could not load your softphone settings.');
      },
    });
  }

  private async registerWith(bootstrap: SoftphoneBootstrap): Promise<void> {
    const engine = this.selectEngine();
    this.engine = engine;

    this.subs.push(
      engine.registrationEvents.subscribe((event) => {
        this._connectState.set(event.state);
        this._connectError.set(event.reason ?? null);
        this.pushLog(
          event.state === 'failed' ? 'error' : 'info',
          `Registration: ${event.state}${event.reason ? ' — ' + event.reason : ''}`,
        );
        if (event.state === 'registered') {
          this.startLivePolling();
        } else {
          this.stopLivePolling();
        }
      }),
    );
    this.subs.push(
      engine.callEvents.subscribe((event) => {
        const isNewCall = !this._calls().some((c) => c.id === event.session.id);
        this._calls.update((calls) => {
          const others = calls.filter((c) => c.id !== event.session.id);
          // A call that ended or failed drops off the list entirely rather
          // than lingering in a terminal state — the panel has nothing
          // useful to show for a call that is over.
          if (event.session.state === 'ended' || event.session.state === 'failed') {
            return others;
          }
          return [...others, event.session];
        });
        this.syncCallLog(event.session, isNewCall);
        const who = event.session.displayName?.trim() || event.session.remote;
        this.pushLog(
          event.session.state === 'failed' ? 'error' : 'info',
          `Call ${event.session.direction} ${who}: ${event.session.state}${event.reason ? ' — ' + event.reason : ''}`,
        );
      }),
    );
    this.subs.push(
      engine.mediaEvents.subscribe((event) => {
        if (event.remoteStream) {
          this._remoteStream.set(event.remoteStream);
        }
      }),
    );

    try {
      await engine.initialize({
        server: bootstrap.server,
        port: bootstrap.port,
        wsPath: bootstrap.ws_path,
        transport: bootstrap.transport === 'ws' ? 'ws' : 'wss',
      });
      await engine.registerAccount({
        id: bootstrap.extension,
        extension: bootstrap.extension,
        displayName: bootstrap.display_name,
        password: bootstrap.password,
      });
    } catch {
      this._connectState.set('error');
      this._connectError.set('Could not connect to the PBX.');
    }
  }

  private selectEngine(): SipEngine {
    // JsSipEngine itself is a thin class; the ~250KB `jssip` package it
    // wraps is dynamically imported inside its own registerAccount() (see
    // that file's doc comment), so choosing it here costs nothing until an
    // agent actually registers.
    return environment.sipEngine === 'jssip' ? new JsSipEngine() : new FakeSipEngine();
  }

  // --- Call-log lifecycle ------------------------------------------------
  //
  // Softphone::createCallLog()/markCallAnswered()/finalizeCallLog()'s
  // browser-side call sites, ported straight from heal-crm's own Alpine
  // component (see default.blade.php's `call:outgoing`/`call:answered`/
  // `call:ended` handlers) but never wired up on this app's side until now
  // — internal/handlers/calllog.go has carried this endpoint set, unused,
  // since it was first ported. Every write here is fire-and-forget: a
  // failed call-log write must never block or break the call itself, it
  // only means the History tab and the call-detail panel's queue badge are
  // missing that one row.

  private syncCallLog(session: CallSession, isNewCall: boolean): void {
    if (isNewCall) {
      const body =
        session.direction === 'inbound'
          ? { caller_id: session.remote, caller_name: session.displayName ?? '', direction: 'incoming' }
          : {
              caller_id: this._bootstrap()?.extension ?? '',
              direction: 'outgoing',
              destination: session.remote,
            };
      this.api
        .post<{ id: string }>('secure/softphone/call-logs', body, { silent: true })
        .subscribe({
          next: (result) => {
            this.updateCallMeta(session.id, { callLogId: result.id });
            if (session.direction === 'inbound') {
              this.resolveQueue(session.id, session.remote, result.id);
            }
          },
          error: () => this.pushLog('error', 'Could not record the call'),
        });
      return;
    }

    const meta = this._callMeta()[session.id];
    if (!meta?.callLogId) {
      return;
    }

    if (session.state === 'answered' && !meta.answeredSent) {
      this.updateCallMeta(session.id, { answeredSent: true });
      this.api
        .post(`secure/softphone/call-logs/${meta.callLogId}/answered`, {}, { silent: true })
        .subscribe({
          error: () => this.pushLog('error', 'Could not mark the call answered'),
        });
    }

    if ((session.state === 'ended' || session.state === 'failed') && !meta.finalizedSent) {
      this.updateCallMeta(session.id, { finalizedSent: true });
      const duration = session.answeredAt
        ? Math.max(0, Math.floor((Date.now() - session.answeredAt.getTime()) / 1000))
        : 0;
      const status = session.answeredAt ? 'completed' : session.direction === 'inbound' ? 'missed' : 'failed';
      this.api
        .post(`secure/softphone/call-logs/${meta.callLogId}/finalize`, { status, duration }, { silent: true })
        .subscribe({
          next: () => this.forgetCallMeta(session.id),
          error: () => this.pushLog('error', 'Could not finalize the call record'),
        });
    }
  }

  private resolveQueue(callId: CallId, callerId: string, callLogId: string): void {
    this.api
      .post<{ queue: string; call_log_id: string }>(
        'secure/softphone/detect-queue',
        { caller_id: callerId, call_log_id: callLogId },
        { silent: true },
      )
      .subscribe({
        next: (result) => {
          if (result.queue) {
            this.updateCallMeta(callId, { queue: result.queue });
          }
        },
        error: () => {
          /* best-effort enrichment only — the panel falls back to the live
             agent snapshot's own active_call.queue. */
        },
      });
  }

  private updateCallMeta(id: CallId, patch: Partial<CallMeta>): void {
    this._callMeta.update((all) => ({
      ...all,
      [id]: {
        callLogId: all[id]?.callLogId ?? null,
        queue: all[id]?.queue ?? null,
        answeredSent: all[id]?.answeredSent ?? false,
        finalizedSent: all[id]?.finalizedSent ?? false,
        ...patch,
      },
    }));
  }

  private forgetCallMeta(id: CallId): void {
    this._callMeta.update((all) => {
      const { [id]: _dropped, ...rest } = all;
      return rest;
    });
  }

  dial(target: string): void {
    void this.engine?.makeCall({ target });
  }

  answer(callId: CallId): void {
    void this.engine?.answerCall(callId);
  }

  decline(callId: CallId): void {
    void this.engine?.rejectCall(callId, 'declined');
  }

  hangup(callId: CallId): void {
    void this.engine?.endCall(callId);
  }

  toggleHold(call: CallSession): void {
    if (call.onHold) {
      void this.engine?.resumeCall(call.id);
    } else {
      void this.engine?.holdCall(call.id);
    }
  }

  toggleMute(call: CallSession): void {
    void this.engine?.muteCall(call.id, !call.muted);
  }

  sendDtmf(callId: CallId, digit: DtmfDigit): void {
    void this.engine?.sendDtmf(callId, digit);
  }

  /** Blind transfer only — see JsSipEngine's own doc comment for why. */
  blindTransfer(callId: CallId, target: string): void {
    void this.engine?.transferCall({ callId, target, kind: 'blind' });
  }

  /** Test-only access to the underlying fake, for driving scenarios from a spec or a dev harness. */
  get fakeEngineForTesting(): FakeSipEngine | null {
    return this.engine instanceof FakeSipEngine ? this.engine : null;
  }

  // --- Queue membership -----------------------------------------------
  //
  // Every endpoint here acts on the CALLER's own membership — see
  // handlers/queuecommand.go's interfaceFor doc comment — so there is no
  // "which agent" parameter to pass from the client side at all.

  queueLogin(queueInput: string): void {
    const queues = parseQueueInput(queueInput);
    this._queueBusy.set(true);
    this._queueError.set(null);
    this.api.post('secure/softphone/queue/login', { queues: queues.length ? queues : ['all'] }).subscribe({
      next: () => {
        this._queueBusy.set(false);
        this.pushLog('info', `Logged into queue(s): ${queues.join(', ') || 'all'}`);
        this.refreshLiveAgents();
      },
      error: (err) => this.failQueueAction(err, 'Could not log into the queue'),
    });
  }

  queueLogout(queueInput: string): void {
    const queues = parseQueueInput(queueInput);
    this._queueBusy.set(true);
    this._queueError.set(null);
    this.api.post('secure/softphone/queue/logout', { queues: queues.length ? queues : ['all'] }).subscribe({
      next: () => {
        this._queueBusy.set(false);
        this.pushLog('info', `Logged out of queue(s): ${queues.join(', ') || 'all'}`);
        this.refreshLiveAgents();
      },
      error: (err) => this.failQueueAction(err, 'Could not log out of the queue'),
    });
  }

  queuePause(reason = ''): void {
    this._queueBusy.set(true);
    this._queueError.set(null);
    this.api.post('secure/softphone/queue/pause', { reason }).subscribe({
      next: () => {
        this._queueBusy.set(false);
        this.pushLog('info', 'Paused');
        this.refreshLiveAgents();
      },
      error: (err) => this.failQueueAction(err, 'Could not pause'),
    });
  }

  queueUnpause(): void {
    this._queueBusy.set(true);
    this._queueError.set(null);
    this.api.post('secure/softphone/queue/unpause', {}).subscribe({
      next: () => {
        this._queueBusy.set(false);
        this.pushLog('info', 'Resumed');
        this.refreshLiveAgents();
      },
      error: (err) => this.failQueueAction(err, 'Could not resume'),
    });
  }

  private failQueueAction(err: unknown, fallback: string): void {
    this._queueBusy.set(false);
    const message = (err as { error?: { message?: string } })?.error?.message || fallback;
    this._queueError.set(message);
    this.pushLog('error', message);
  }

  // --- Supervisor actions -----------------------------------------------
  //
  // Gated server-side by softphone.supervise (see router.go) — the panel
  // only offers these when bootstrap.is_supervisor is true, but the server
  // is what actually enforces it.

  /** Monitor/whisper/barge into targetExt's live call. The PBX originates
   * the resulting audio leg back to THIS supervisor's own extension, so it
   * arrives here as an ordinary incoming call — no separate UI surface
   * needed for "listening", the softphone panel's normal in-call card
   * handles it. */
  spy(targetExt: string, mode: SpyMode): void {
    this.api.post('secure/softphone/queue/spy', { target_ext: targetExt, mode }).subscribe({
      next: () => this.pushLog('info', `${mode} started on ${targetExt}`),
      error: (err) => this.pushLog('error', extractError(err, `Could not ${mode} ${targetExt}`)),
    });
  }

  /** Rings a waiting queue call directly at this agent's own extension. */
  pickupQueueCall(channel: string, agentExt: string): void {
    this.api.post('secure/softphone/queue/pickup', { channel, agent_ext: agentExt }).subscribe({
      next: () => this.pushLog('info', `Picked up call on ${channel}`),
      error: (err) => this.pushLog('error', extractError(err, 'Could not pick up that call')),
    });
  }

  /** Blind-transfers a waiting (not yet answered) queue call to extension. */
  redirectQueueCall(channel: string, extension: string): void {
    this.api.post('secure/softphone/queue/redirect', { channel, extension }).subscribe({
      next: () => this.pushLog('info', `Redirected call to ${extension}`),
      error: (err) => this.pushLog('error', extractError(err, 'Could not redirect that call')),
    });
  }

  // --- Live snapshots ----------------------------------------------------

  private startLivePolling(): void {
    this.stopLivePolling();
    this.refreshLiveQueues();
    this.refreshLiveAgents();
    this.pollTimer = setInterval(() => {
      this.refreshLiveQueues();
      this.refreshLiveAgents();
    }, 6000);
  }

  private stopLivePolling(): void {
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }

  refreshLiveQueues(): void {
    this.api
      .get<LiveSnapshotEnvelope<LiveQueuesSnapshot>>('secure/softphone/queues', undefined, {
        silent: true,
      })
      .subscribe({
        next: (envelope) => this._liveQueues.set(envelope.data),
        error: () => {
          /* transient — the next poll tries again. */
        },
      });
  }

  refreshLiveAgents(): void {
    this.api
      .get<LiveSnapshotEnvelope<LiveAgentsSnapshot>>('secure/softphone/agents', undefined, {
        silent: true,
      })
      .subscribe({
        next: (envelope) => this._liveAgents.set(envelope.data),
        error: () => {
          /* transient — the next poll tries again. */
        },
      });
  }

  // --- History (this agent's own past calls) -----------------------------

  loadHistory(page = 1, perPage = 25): void {
    this.api.list<CallLogEntry>('secure/softphone/call-logs/mine', { page, per_page: perPage }).subscribe({
      next: (result) => {
        this._historyEntries.set(result.rows);
        this._historyMeta.set(result.meta);
      },
      error: () => this.pushLog('error', 'Could not load call history'),
    });
  }

  // --- Diagnostic log (client-only, never sent to the server) ------------

  /**
   * Capped at 300 so a long shift doesn't grow this without bound; newest
   * first, matching heal-crm's own `debugLogs` (also purely in-memory,
   * cleared on reload — see default.blade.php's own comment on it).
   */
  private pushLog(level: DebugLevel, message: string): void {
    this._logs.update((list) => {
      const entry: DebugLogEntry = { id: ++this.logSeq, at: new Date(), level, message };
      const next = [entry, ...list];
      return next.length > 300 ? next.slice(0, 300) : next;
    });
  }

  clearLogs(): void {
    this._logs.set([]);
  }

  disconnect(): void {
    this.subs.forEach((s) => s.unsubscribe());
    this.subs = [];
    void this.engine?.dispose();
    this.engine = null;
    this.stopLivePolling();
    this._connectState.set('idle');
    this._calls.set([]);
    this._remoteStream.set(null);
    this._liveQueues.set(null);
    this._liveAgents.set(null);
    this._callMeta.set({});
  }
}

/** "4000|5000", "4000, 5000", "4000 5000" — heal-crm accepts all three
 * separators (see default.blade.php's own normalizeQueueInput); this does
 * the same, deduplicated, empty-input meaning "every queue I'm already a
 * member of" (the caller substitutes ['all'] when this returns []). */
function parseQueueInput(raw: string): string[] {
  const parts = raw
    .split(/[|,;\s]+/)
    .map((s) => s.trim())
    .filter(Boolean);
  return Array.from(new Set(parts));
}

function extractError(err: unknown, fallback: string): string {
  return (err as { error?: { message?: string } })?.error?.message || fallback;
}
