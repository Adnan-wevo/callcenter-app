import { Observable, Subject, EMPTY } from 'rxjs';

import {
  CallEvent,
  CallId,
  CallRejectReason,
  CallRequest,
  CallSession,
  CallTransferRequest,
  DtmfDigit,
  EngineDiagnostic,
  MediaEvent,
  RegistrationEvent,
  SipAccount,
  SipEngine,
  SipEngineConfig,
} from '../domain/sip-engine';

/**
 * An in-memory SipEngine that never touches a network.
 *
 * It exists for the reason the Flutter app's own `test_fake` engine does:
 * call control is stateful and order-dependent, and needing a live Asterisk
 * to exercise it is how the state machine stays broken. With this, the whole
 * UI — ringing, answer, hold, mute, transfer, hangup — can be driven in a
 * unit test or clicked through locally with no PBX at all.
 *
 * It models the state TRANSITIONS faithfully and nothing else: there is no
 * audio, no SIP, no negotiation. A test that passes here proves the app's
 * logic is coherent, not that the PBX agrees with it.
 */
export class FakeSipEngine implements SipEngine {
  private readonly registration$ = new Subject<RegistrationEvent>();
  private readonly call$ = new Subject<CallEvent>();
  private readonly media$ = new Subject<MediaEvent>();

  readonly registrationEvents: Observable<RegistrationEvent> = this.registration$.asObservable();
  readonly callEvents: Observable<CallEvent> = this.call$.asObservable();
  readonly mediaEvents: Observable<MediaEvent> = this.media$.asObservable();
  /** Nothing to diagnose without a real transport. */
  readonly diagnostics: Observable<EngineDiagnostic> = EMPTY;

  private readonly sessions = new Map<CallId, CallSession>();
  private nextId = 1;

  async initialize(_config: SipEngineConfig): Promise<void> {
    /* nothing to set up */
  }

  async registerAccount(account: SipAccount): Promise<void> {
    this.registration$.next({ accountId: account.id, state: 'registering' });
    this.registration$.next({ accountId: account.id, state: 'registered' });
  }

  async unregisterAccount(accountId: string): Promise<void> {
    this.registration$.next({ accountId, state: 'unregistered' });
  }

  async makeCall(request: CallRequest): Promise<CallSession> {
    const session: CallSession = {
      id: `fake-${this.nextId++}`,
      direction: 'outbound',
      remote: request.target,
      state: 'progress',
      muted: false,
      onHold: false,
    };
    this.sessions.set(session.id, session);
    this.call$.next({ session: { ...session } });
    return { ...session };
  }

  /**
   * Simulate an inbound call. Not part of SipEngine — a test or a dev
   * harness calls this to make the phone ring.
   */
  simulateIncoming(remote: string, displayName?: string): CallSession {
    const session: CallSession = {
      id: `fake-${this.nextId++}`,
      direction: 'inbound',
      remote,
      displayName,
      state: 'ringing',
      muted: false,
      onHold: false,
    };
    this.sessions.set(session.id, session);
    this.call$.next({ session: { ...session } });
    return { ...session };
  }

  /** Simulate the far end picking up an outbound call. */
  simulateAnsweredByRemote(callId: CallId): void {
    this.transition(callId, (s) => {
      s.state = 'answered';
      s.answeredAt = new Date();
    });
  }

  async answerCall(callId: CallId): Promise<void> {
    this.transition(callId, (s) => {
      s.state = 'answered';
      s.answeredAt = new Date();
    });
  }

  async rejectCall(callId: CallId, reason?: CallRejectReason): Promise<void> {
    this.transition(callId, (s) => (s.state = 'ended'), reason);
    this.sessions.delete(callId);
  }

  async endCall(callId: CallId): Promise<void> {
    this.transition(callId, (s) => (s.state = 'ended'));
    this.sessions.delete(callId);
  }

  async holdCall(callId: CallId): Promise<void> {
    this.transition(callId, (s) => {
      s.onHold = true;
      s.state = 'held';
    });
  }

  async resumeCall(callId: CallId): Promise<void> {
    this.transition(callId, (s) => {
      s.onHold = false;
      s.state = 'answered';
    });
  }

  async muteCall(callId: CallId, enabled: boolean): Promise<void> {
    this.transition(callId, (s) => (s.muted = enabled));
  }

  async sendDtmf(_callId: CallId, _digit: DtmfDigit): Promise<void> {
    /* nothing audible to send */
  }

  async transferCall(request: CallTransferRequest): Promise<void> {
    // A blind transfer ends the call for this party immediately; an attended
    // one would keep it until the consultation completes. The fake models
    // the blind case only, which is the one the callback flow uses.
    this.transition(request.callId, (s) => (s.state = 'ended'));
    this.sessions.delete(request.callId);
  }

  async dispose(): Promise<void> {
    this.sessions.clear();
    this.registration$.complete();
    this.call$.complete();
    this.media$.complete();
  }

  /**
   * Mutate a session and emit it. Emitting a COPY matters: handing out the
   * live object would let a subscriber mutate engine state by accident, and
   * a later comparison against "the previous session" would find the same
   * object changed under it.
   */
  private transition(callId: CallId, mutate: (s: CallSession) => void, reason?: string): void {
    const session = this.sessions.get(callId);
    if (!session) {
      return;
    }
    mutate(session);
    this.call$.next({ session: { ...session }, reason });
  }
}
