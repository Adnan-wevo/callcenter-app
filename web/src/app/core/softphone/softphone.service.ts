import { Injectable, computed, inject, signal } from '@angular/core';
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
import { SoftphoneBootstrap } from './softphone.types';

export type ConnectState = 'idle' | 'connecting' | RegistrationState | 'no-extension' | 'error';

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

  readonly connectState = this._connectState.asReadonly();
  readonly connectError = this._connectError.asReadonly();
  readonly bootstrap = this._bootstrap.asReadonly();
  readonly calls = this._calls.asReadonly();
  /** The far end's audio, for the panel to attach to an <audio> element. */
  readonly remoteStream = this._remoteStream.asReadonly();

  readonly isConnected = computed(() => this._connectState() === 'registered');

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
      }),
    );
    this.subs.push(
      engine.callEvents.subscribe((event) => {
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

  disconnect(): void {
    this.subs.forEach((s) => s.unsubscribe());
    this.subs = [];
    void this.engine?.dispose();
    this.engine = null;
    this._connectState.set('idle');
    this._calls.set([]);
    this._remoteStream.set(null);
  }
}
