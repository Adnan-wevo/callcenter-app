import { Observable, Subject } from 'rxjs';

import {
  CallEvent,
  CallId,
  CallRejectReason,
  CallRequest,
  CallSession,
  CallTransferRequest,
  DtmfDigit,
  MediaEvent,
  RegistrationEvent,
  SipAccount,
  SipEngine,
  SipEngineConfig,
} from '../domain/sip-engine';

// JsSIP ships no useful types for the RTCSession surface, so it is imported
// untyped and narrowed locally. Widening `any` across this file would push
// the looseness into the rest of the app; keeping it at the boundary means
// everything outside sees the typed SipEngine contract instead.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type JsSipRTCSession = any;

/**
 * The real engine: a SIP user agent in the browser, registered to Asterisk
 * over a secure WebSocket, with WebRTC carrying the audio.
 *
 * Same library heal-crm uses today, so the PBX sees a client it already
 * knows how to talk to. What differs is the shape around it: this sits
 * behind the SipEngine interface, so the UI never touches JsSIP and the
 * whole call flow can be exercised against FakeSipEngine with no PBX.
 *
 * # Media
 *
 * Audio only. `getUserMedia({ audio: true })` prompts for the microphone the
 * first time and the browser remembers the grant per origin — which is one
 * more reason the app must be served over HTTPS in any real deployment: on
 * plain HTTP the API is not available at all outside localhost.
 */
export class JsSipEngine implements SipEngine {
  private readonly registration$ = new Subject<RegistrationEvent>();
  private readonly call$ = new Subject<CallEvent>();
  private readonly media$ = new Subject<MediaEvent>();

  readonly registrationEvents: Observable<RegistrationEvent> = this.registration$.asObservable();
  readonly callEvents: Observable<CallEvent> = this.call$.asObservable();
  readonly mediaEvents: Observable<MediaEvent> = this.media$.asObservable();

  private config?: SipEngineConfig;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private ua?: any;
  private accountId?: string;

  private readonly sessions = new Map<CallId, CallSession>();
  private readonly rtc = new Map<CallId, JsSipRTCSession>();
  private nextId = 1;

  async initialize(config: SipEngineConfig): Promise<void> {
    this.config = config;
  }

  async registerAccount(account: SipAccount): Promise<void> {
    if (!this.config) {
      throw new Error('JsSipEngine: initialize() must be called before registerAccount()');
    }

    // Imported lazily so JsSIP (≈250KB) is not in the initial bundle: a
    // supervisor who only reads reports should not pay for a phone they
    // never open.
    const JsSIP = (await import('jssip')).default;

    const { server, port, wsPath, transport } = this.config;
    const socket = new JsSIP.WebSocketInterface(`${transport}://${server}:${port}${wsPath}`);

    this.accountId = account.id;
    this.ua = new JsSIP.UA({
      sockets: [socket],
      uri: `sip:${account.extension}@${server}`,
      password: account.password,
      display_name: account.displayName,
      // Asterisk drops a registration it has not heard from; re-registering
      // well inside the default 3600s expiry keeps the phone reachable
      // through a brief network blip rather than silently going dead.
      register_expires: 300,
    });

    this.ua.on('connecting', () => this.emitRegistration('registering'));
    this.ua.on('registered', () => this.emitRegistration('registered'));
    this.ua.on('unregistered', () => this.emitRegistration('unregistered'));
    this.ua.on('registrationFailed', (e: { cause?: string }) =>
      this.emitRegistration('failed', e?.cause ?? 'registration rejected'),
    );
    this.ua.on('disconnected', () =>
      this.emitRegistration('failed', 'the connection to the PBX dropped'),
    );

    this.ua.on('newRTCSession', ({ session }: { session: JsSipRTCSession }) => {
      if (session.direction === 'incoming') {
        this.adoptIncoming(session);
      }
    });

    this.ua.start();
  }

  async unregisterAccount(_accountId: string): Promise<void> {
    this.ua?.stop();
  }

  async makeCall(request: CallRequest): Promise<CallSession> {
    if (!this.ua || !this.config) {
      throw new Error('JsSipEngine: not registered');
    }

    const id = this.newId();
    const session: CallSession = {
      id,
      direction: 'outbound',
      remote: request.target,
      state: 'progress',
      muted: false,
      onHold: false,
    };
    this.sessions.set(id, session);

    const rtc = this.ua.call(`sip:${request.target}@${this.config.server}`, {
      mediaConstraints: { audio: true, video: false },
      // A call with no inbound audio is a call the agent cannot hear. Asking
      // for it explicitly beats relying on the far end to offer it.
      rtcOfferConstraints: { offerToReceiveAudio: true, offerToReceiveVideo: false },
    });

    this.rtc.set(id, rtc);
    this.bind(id, rtc);
    this.emitCall(id);

    return { ...session };
  }

  async answerCall(callId: CallId): Promise<void> {
    this.rtc.get(callId)?.answer({ mediaConstraints: { audio: true, video: false } });
  }

  async rejectCall(callId: CallId, reason?: CallRejectReason): Promise<void> {
    // SIP status codes: 486 Busy Here, 603 Decline, 480 Temporarily
    // Unavailable. The far end (and the queue) treats these differently, so
    // the distinction is worth carrying rather than hanging up on everything.
    const status = reason === 'busy' ? 486 : reason === 'unavailable' ? 480 : 603;
    this.rtc.get(callId)?.terminate({ status_code: status });
  }

  async endCall(callId: CallId): Promise<void> {
    this.rtc.get(callId)?.terminate();
  }

  async holdCall(callId: CallId): Promise<void> {
    this.rtc.get(callId)?.hold();
    this.update(callId, (s) => {
      s.onHold = true;
      s.state = 'held';
    });
  }

  async resumeCall(callId: CallId): Promise<void> {
    this.rtc.get(callId)?.unhold();
    this.update(callId, (s) => {
      s.onHold = false;
      s.state = 'answered';
    });
  }

  async muteCall(callId: CallId, enabled: boolean): Promise<void> {
    const rtc = this.rtc.get(callId);
    if (!rtc) {
      return;
    }
    if (enabled) {
      rtc.mute({ audio: true });
    } else {
      rtc.unmute({ audio: true });
    }
    this.update(callId, (s) => (s.muted = enabled));
  }

  async sendDtmf(callId: CallId, digit: DtmfDigit): Promise<void> {
    this.rtc.get(callId)?.sendDTMF(digit);
  }

  async transferCall(request: CallTransferRequest): Promise<void> {
    const rtc = this.rtc.get(request.callId);
    if (!rtc || !this.config) {
      return;
    }
    // Blind only for now. An attended transfer needs a second call and a
    // REFER referencing it — a different flow, not a flag on this one, so it
    // is left unimplemented rather than faked.
    if (request.kind === 'attended') {
      throw new Error('JsSipEngine: attended transfer is not implemented yet');
    }
    rtc.refer(`sip:${request.target}@${this.config.server}`);
  }

  async dispose(): Promise<void> {
    for (const rtc of this.rtc.values()) {
      try {
        rtc.terminate();
      } catch {
        // Already gone; disposing must not throw.
      }
    }
    this.rtc.clear();
    this.sessions.clear();
    this.ua?.stop();
    this.registration$.complete();
    this.call$.complete();
    this.media$.complete();
  }

  private adoptIncoming(rtc: JsSipRTCSession): void {
    const id = this.newId();
    this.sessions.set(id, {
      id,
      direction: 'inbound',
      remote: rtc.remote_identity?.uri?.user ?? 'unknown',
      displayName: rtc.remote_identity?.display_name ?? undefined,
      state: 'ringing',
      muted: false,
      onHold: false,
    });
    this.rtc.set(id, rtc);
    this.bind(id, rtc);
    this.emitCall(id);
  }

  private bind(id: CallId, rtc: JsSipRTCSession): void {
    rtc.on('confirmed', () =>
      this.update(id, (s) => {
        s.state = 'answered';
        s.answeredAt = new Date();
      }),
    );

    rtc.on('failed', (e: { cause?: string }) => {
      this.update(id, (s) => (s.state = 'failed'), e?.cause);
      this.forget(id);
    });

    rtc.on('ended', () => {
      this.update(id, (s) => (s.state = 'ended'));
      this.forget(id);
    });

    // The remote audio arrives on a track event; the page attaches it to an
    // <audio> element. Handing over the stream rather than playing it here
    // keeps this engine free of DOM.
    rtc.connection?.addEventListener?.('track', (event: RTCTrackEvent) => {
      const [remoteStream] = event.streams;
      if (remoteStream) {
        this.media$.next({ callId: id, remoteStream });
      }
    });
  }

  private forget(id: CallId): void {
    this.rtc.delete(id);
    this.sessions.delete(id);
  }

  private newId(): CallId {
    return `call-${this.nextId++}`;
  }

  private emitRegistration(state: RegistrationEvent['state'], reason?: string): void {
    this.registration$.next({ accountId: this.accountId ?? '', state, reason });
  }

  private emitCall(id: CallId, reason?: string): void {
    const session = this.sessions.get(id);
    if (session) {
      // A copy, so a subscriber cannot mutate engine state by accident.
      this.call$.next({ session: { ...session }, reason });
    }
  }

  private update(id: CallId, mutate: (s: CallSession) => void, reason?: string): void {
    const session = this.sessions.get(id);
    if (!session) {
      return;
    }
    mutate(session);
    this.emitCall(id, reason);
  }
}
