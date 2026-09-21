import { Observable } from 'rxjs';

/**
 * The SIP engine contract.
 *
 * Deliberately modelled on `wevetel_omnisoftphone`'s own
 * `lib/engines/sip/domain/sip_engine.dart`, with Dart `Stream`s becoming
 * RxJS `Observable`s and `Future`s becoming `Promise`s. Two reasons to
 * follow that shape rather than invent one:
 *
 *   - **It is already the house pattern.** An engineer who knows the Flutter
 *     app can read this, and a behaviour fixed in one has an obvious
 *     counterpart in the other.
 *   - **It makes the call logic testable without a PBX.** The Flutter app
 *     ships a `test_fake` engine beside its real ones; so does this
 *     (`FakeSipEngine`). Call-control logic is stateful, order-dependent and
 *     the single hardest thing here to get right — being unable to exercise
 *     it without live Asterisk is how it stays wrong.
 *
 * The engine owns SIP and media. It knows nothing about this application's
 * screens, permissions or reports.
 */

export type CallId = string;
export type AccountId = string;

export type RegistrationState = 'unregistered' | 'registering' | 'registered' | 'failed';

export interface RegistrationEvent {
  accountId: AccountId;
  state: RegistrationState;
  /** Present when state is 'failed'. */
  reason?: string;
}

/**
 * The lifecycle of one call.
 *
 * `ringing` is an INBOUND call not yet answered; `progress` is an outbound
 * one that has not been picked up. They are separate states because the UI
 * and the permitted actions differ — you may answer one and not the other.
 */
export type CallState =
  | 'idle'
  | 'progress'
  | 'ringing'
  | 'answered'
  | 'held'
  | 'ended'
  | 'failed';

export type CallDirection = 'inbound' | 'outbound';

export interface CallSession {
  id: CallId;
  direction: CallDirection;
  /** The far end: a number for outbound, the caller id for inbound. */
  remote: string;
  displayName?: string;
  state: CallState;
  /** When the call was answered, for the in-call timer. Absent until then. */
  answeredAt?: Date;
  muted: boolean;
  onHold: boolean;
}

export interface CallEvent {
  session: CallSession;
  /** Present when the state is 'failed' or 'ended' for a non-normal reason. */
  reason?: string;
}

export interface MediaEvent {
  callId: CallId;
  /** The remote audio, for the page to attach to an <audio> element. */
  remoteStream?: MediaStream;
}

export interface SipAccount {
  id: AccountId;
  /** The SIP extension, e.g. "1001". */
  extension: string;
  displayName: string;
  password: string;
}

export interface SipEngineConfig {
  /** PBX host, without scheme or port. */
  server: string;
  port: number;
  /** WebSocket path on the PBX, e.g. "/ws". */
  wsPath: string;
  /** "wss" in any real deployment; "ws" is for a local PBX only. */
  transport: 'ws' | 'wss';
}

export type CallRejectReason = 'busy' | 'declined' | 'unavailable';

export interface CallRequest {
  /** The number or extension to dial. */
  target: string;
}

export type TransferKind = 'blind' | 'attended';

export interface CallTransferRequest {
  callId: CallId;
  target: string;
  kind: TransferKind;
}

export type DtmfDigit =
  | '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9'
  | '*' | '#';

/**
 * Implemented once per transport. `JsSipEngine` is the real one (WebRTC over
 * a secure WebSocket to Asterisk); `FakeSipEngine` is the in-memory stand-in.
 */
export interface SipEngine {
  readonly registrationEvents: Observable<RegistrationEvent>;
  readonly callEvents: Observable<CallEvent>;
  readonly mediaEvents: Observable<MediaEvent>;

  initialize(config: SipEngineConfig): Promise<void>;

  registerAccount(account: SipAccount): Promise<void>;
  unregisterAccount(accountId: AccountId): Promise<void>;

  makeCall(request: CallRequest): Promise<CallSession>;
  answerCall(callId: CallId): Promise<void>;
  rejectCall(callId: CallId, reason?: CallRejectReason): Promise<void>;
  endCall(callId: CallId): Promise<void>;

  holdCall(callId: CallId): Promise<void>;
  resumeCall(callId: CallId): Promise<void>;
  muteCall(callId: CallId, enabled: boolean): Promise<void>;
  sendDtmf(callId: CallId, digit: DtmfDigit): Promise<void>;
  transferCall(request: CallTransferRequest): Promise<void>;

  dispose(): Promise<void>;
}
