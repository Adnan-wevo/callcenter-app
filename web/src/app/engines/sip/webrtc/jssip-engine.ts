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
 * No ICE server was configured anywhere in this codebase before — confirmed
 * by search, and the reason a WebRTC call would connect (SDP/SIP signalling
 * succeeds) and then die on its own a few dozen seconds later: without a
 * STUN server, a browser behind NAT can only gather a `host` ICE candidate,
 * which the PBX on the public internet usually cannot reach. JsSIP's own
 * RTCSession wires `iceconnectionstatechange` to auto-`terminate()` the
 * call the moment `iceConnectionState` becomes `'failed'` — which is
 * exactly "the call dies by itself", not a crash anywhere in this app's own
 * code. A public STUN server fixes the common case (NAT with a normal,
 * non-symmetric mapping); a fully locked-down network still needs a TURN
 * relay, which this does not attempt to provide.
 */
const ICE_SERVERS: RTCIceServer[] = [
  { urls: ['stun:stun.l.google.com:19302', 'stun:stun1.l.google.com:19302'] },
];

/**
 * For a queue-routed call, Asterisk's dialplan sets the INVITE's From
 * display-name to the queue number glued directly onto the caller's own
 * number with no separator (queue 50000 + caller 0198202884 ->
 * "500000198202884") — that is routing metadata for the agent leg, not a
 * human name, but JsSIP hands it straight through as
 * `remote_identity.display_name` with nothing to mark it as different from
 * a real Caller ID name. The queue is already shown separately (see
 * `primaryCallQueue`), so anything that's purely digits and ends with the
 * caller's own number is treated as no name at all instead of being shown
 * as one — and, since `CallSession.displayName` is what gets sent as
 * `caller_name` when the call log is created, this also keeps that junk
 * value out of call history.
 */
function cleanDisplayName(raw: string | undefined, remote: string): string | undefined {
  const name = raw?.trim();
  if (!name) {
    return undefined;
  }
  return /^\d+$/.test(name) && name.endsWith(remote) ? undefined : name;
}

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
      pcConfig: { iceServers: ICE_SERVERS },
    });

    this.rtc.set(id, rtc);
    this.bind(id, rtc);
    this.emitCall(id);

    return { ...session };
  }

  async answerCall(callId: CallId): Promise<void> {
    this.rtc.get(callId)?.answer({
      mediaConstraints: { audio: true, video: false },
      pcConfig: { iceServers: ICE_SERVERS },
    });
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
    const remote = rtc.remote_identity?.uri?.user ?? 'unknown';
    this.sessions.set(id, {
      id,
      direction: 'inbound',
      remote,
      displayName: cleanDisplayName(rtc.remote_identity?.display_name, remote),
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
    //
    // WHEN the underlying RTCPeerConnection exists differs by direction, and
    // that used to break inbound audio entirely:
    //   - Outbound: ua.call() -> RTCSession.connect() creates it
    //     SYNCHRONOUSLY, before this bind() call even runs. `rtc.connection`
    //     already exists here.
    //   - Inbound: JsSIP only creates it inside answer() (RTCSession's
    //     _createRTCConnection, called from init_incoming's answer path),
    //     which runs much later — once the agent actually clicks Answer.
    //     `rtc.connection` is `undefined` at bind() time.
    // The old code read `rtc.connection?.addEventListener?.(...)` once,
    // here — correct for outbound (connection already exists), silently a
    // no-op for inbound (nothing to attach to yet, and nothing ever
    // retried), so no 'track' listener was EVER attached for an inbound
    // call and remote audio never arrived. JsSIP's own 'peerconnection'
    // event fires the instant the connection is created, for both
    // directions — but subscribing to it unconditionally reintroduces the
    // same bug in the other direction, because for outbound that event has
    // already fired (synchronously, above) by the time we get here, and an
    // event fired before a listener subscribes is simply missed. Covering
    // both requires both: attach immediately if the connection already
    // exists, otherwise wait for it to be created.
    const attachTrackListener = (peerconnection: RTCPeerConnection) => {
      peerconnection.addEventListener('track', (event: RTCTrackEvent) => {
        const [remoteStream] = event.streams;
        if (remoteStream) {
          this.media$.next({ callId: id, remoteStream });
        }
      });
    };
    if (rtc.connection) {
      attachTrackListener(rtc.connection);
    } else {
      rtc.on('peerconnection', ({ peerconnection }: { peerconnection: RTCPeerConnection }) =>
        attachTrackListener(peerconnection),
      );
    }
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
