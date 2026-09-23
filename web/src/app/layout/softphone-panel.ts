import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  effect,
  inject,
  signal,
  viewChild,
} from '@angular/core';

import { CallSession } from '../engines/sip/domain/sip-engine';
import { SoftphoneService } from '../core/softphone/softphone.service';
import { IconComponent } from '../shared/components/icon/icon';
import { ContactsTabComponent } from './softphone/contacts-tab';
import { HistoryTabComponent } from './softphone/history-tab';
import { LogsTabComponent } from './softphone/logs-tab';
import { QueueTabComponent } from './softphone/queue-tab';
import { SupervisorPanelComponent } from './softphone/supervisor-panel';

type SoftphoneTab = 'queue' | 'contacts' | 'history' | 'logs';

const DIALPAD_KEYS = [
  ['1', ''], ['2', 'ABC'], ['3', 'DEF'],
  ['4', 'GHI'], ['5', 'JKL'], ['6', 'MNO'],
  ['7', 'PQRS'], ['8', 'TUV'], ['9', 'WXYZ'],
  ['*', ''], ['0', '+'], ['#', ''],
] as const;

/**
 * The softphone: registration status, a dialpad, and the active call(s).
 *
 * # Why this is a `layout/` component, not a `feature/`
 *
 * It has to survive route navigation — an agent switching from Dashboard to
 * Unanswered Calls mid-call must not lose the call. A routed component is
 * destroyed on navigation; this one is mounted once by `app.html`, beside
 * the sidebar, for the lifetime of the session. `SoftphoneService` itself is
 * `providedIn: 'root'` for the same reason: the state has to outlive
 * whichever screen is showing.
 *
 * This is the Angular shape of what heal-crm does with
 * `@persist('softphone-panel')` on its Livewire component — same problem
 * (a call must survive navigation), solved with this framework's own tool
 * for it rather than a port of Livewire's.
 *
 * # What this does NOT do
 *
 * It drives whichever `SipEngine` `SoftphoneService` has selected — the real
 * one or `FakeSipEngine` — and knows nothing about which.
 *
 * Not everything on screen goes through that engine, though. Conference and
 * attended transfer run over the PBX instead: a browser cannot mix three
 * audio streams, and a SIP REFER cannot express "let me speak to them
 * first". Supervisor monitor/whisper/barge is the same story
 * (`app-supervisor-panel`, below) — the PBX originates the resulting audio
 * leg back to the supervisor's own extension as an ordinary incoming call,
 * so the engine never knows it is a spy session rather than a real one.
 */
@Component({
  selector: 'app-softphone-panel',
  standalone: true,
  imports: [
    IconComponent,
    SupervisorPanelComponent,
    QueueTabComponent,
    ContactsTabComponent,
    HistoryTabComponent,
    LogsTabComponent,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './softphone-panel.html',
})
export class SoftphonePanelComponent {
  protected readonly phone = inject(SoftphoneService);

  protected readonly dialpadKeys = DIALPAD_KEYS;

  /** Collapsed by default on a narrow viewport; open on desktop, matching heal-crm's own default. */
  protected readonly open = signal(window.matchMedia('(min-width: 1024px)').matches);

  protected readonly dialInput = signal('');
  protected readonly showKeypadDuringCall = signal(false);
  protected readonly showTransferInput = signal(false);
  protected readonly transferTarget = signal('');
  protected readonly showConferenceInput = signal(false);
  protected readonly conferenceTarget = signal('');
  protected readonly activeTab = signal<SoftphoneTab>('queue');

  protected readonly historyCount = computed(() => this.phone.historyMeta()?.total ?? 0);

  protected readonly callStateLabel = computed(() => {
    const call = this.phone.primaryCall();
    if (!call) {
      return 'IDLE';
    }
    return call.state === 'progress' ? 'CALLING' : 'IN CALL';
  });

  protected readonly tabs = computed(() => [
    { id: 'queue' as const, label: 'Queue', count: this.phone.queueWaitingCalls().length },
    { id: 'contacts' as const, label: 'Contacts', count: this.phone.contacts().length },
    { id: 'history' as const, label: 'History', count: this.historyCount() },
    { id: 'logs' as const, label: 'Logs', count: this.phone.logs().length },
  ]);

  private readonly audioEl = viewChild<ElementRef<HTMLAudioElement>>('remoteAudio');

  constructor() {
    // Attach the remote party's audio as soon as both the <audio> element and
    // a stream exist. An effect rather than a template binding, because
    // `srcObject` is not an attribute React/Angular can bind declaratively —
    // it must be set on the element property directly.
    effect(() => {
      const el = this.audioEl()?.nativeElement;
      const stream = this.phone.remoteStream();
      if (el) {
        el.srcObject = stream;
      }
    });
  }

  protected toggleOpen(): void {
    this.open.set(!this.open());
  }

  protected pressDigit(digit: string): void {
    const call = this.phone.primaryCall();
    if (call && this.showKeypadDuringCall()) {
      this.phone.sendDtmf(call.id, digit as never);
      return;
    }
    this.dialInput.update((v) => v + digit);
  }

  protected backspace(): void {
    this.dialInput.update((v) => v.slice(0, -1));
  }

  protected callDialInput(): void {
    const target = this.dialInput().trim();
    if (!target) {
      return;
    }
    this.phone.dial(target);
    this.dialInput.set('');
  }

  protected answer(call: CallSession): void {
    this.phone.answer(call.id);
  }

  protected decline(call: CallSession): void {
    this.phone.decline(call.id);
  }

  protected hangup(call: CallSession): void {
    this.phone.hangup(call.id);
  }

  protected toggleHold(call: CallSession): void {
    this.phone.toggleHold(call);
  }

  protected toggleMute(call: CallSession): void {
    this.phone.toggleMute(call);
  }

  /** Hand the call over immediately, with no confirmation leg — the agent
   * is out of it as soon as the PBX accepts the REFER. */
  protected transfer(call: CallSession): void {
    const target = this.transferTarget().trim();
    if (!target) {
      return;
    }
    this.phone.blindTransfer(call.id, target);
    this.transferTarget.set('');
    this.showTransferInput.set(false);
  }

  /** Speak to the target first, with the caller on hold. Runs over the PBX
   * rather than the SIP engine: a REFER cannot express a consultation. */
  protected consult(): void {
    const target = this.transferTarget().trim();
    if (!target) {
      return;
    }
    this.phone.attendedTransfer(target);
    this.transferTarget.set('');
    this.showTransferInput.set(false);
  }

  protected addToConference(): void {
    const target = this.conferenceTarget().trim();
    if (!target) {
      return;
    }
    this.phone.conferenceStart(target);
    this.conferenceTarget.set('');
    this.showConferenceInput.set(false);
  }

  /**
   * Finish an attended transfer by hanging up this agent's own leg.
   *
   * That IS the completion step — Asterisk's Atxfer hands the caller to the
   * target when the transferring agent drops out, which is why v3 exposes
   * no "complete" endpoint (see its TransferService.Attended, whose CDR
   * note says the row is written "when the agent eventually hangs up").
   * It needs its own button regardless: reaching for the red hangup to
   * complete a transfer reads as abandoning the caller, so an agent who is
   * not told simply will not do it.
   */
  protected completeTransfer(call: CallSession): void {
    this.phone.hangup(call.id);
  }

  protected toggleConferenceMute(): void {
    this.phone.conferenceMute(!this.phone.conferenceMuted());
  }

  protected retry(): void {
    this.phone.connect();
  }

  /** The caller's display name, falling back to the raw number. */
  protected label(call: CallSession): string {
    return call.displayName?.trim() || call.remote;
  }
}
