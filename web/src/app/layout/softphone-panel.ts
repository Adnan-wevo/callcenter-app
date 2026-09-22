import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
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
 * one or `FakeSipEngine` — and knows nothing about which. Attended transfer,
 * conference and supervisor spy/whisper/barge are not here yet; see
 * `SipEngine`'s own doc comment for what is deliberately unimplemented and
 * why.
 */
@Component({
  selector: 'app-softphone-panel',
  standalone: true,
  imports: [IconComponent],
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

  private readonly audioEl = viewChild<ElementRef<HTMLAudioElement>>('remoteAudio');

  /**
   * The call the panel treats as "the" call for the single-call controls
   * (mute/hold/hangup): the most recently changed active call. Concurrent
   * calls beyond this one (call waiting) are listed but not the primary
   * focus — multi-call juggling is out of scope for this first version.
   */
  protected readonly primaryCall = computed<CallSession | undefined>(() => {
    const calls = this.phone.activeCalls();
    return calls[calls.length - 1];
  });

  // A local timer tick, the same pattern app.ts's clock uses: a signal ticked
  // by an interval rather than a pipe, so in-call duration updates without a
  // change-detection pass over the whole tree. Only runs while a call is
  // actually answered — a collapsed, idle panel should not be doing anything
  // every second.
  private readonly now = signal(Date.now());

  protected readonly elapsed = computed(() => {
    const call = this.primaryCall();
    if (!call?.answeredAt) {
      return null;
    }
    const seconds = Math.max(0, Math.floor((this.now() - call.answeredAt.getTime()) / 1000));
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  });

  constructor() {
    let tick: ReturnType<typeof setInterval> | null = null;
    effect(() => {
      const answered = this.primaryCall()?.answeredAt != null;
      if (answered && !tick) {
        tick = setInterval(() => this.now.set(Date.now()), 1000);
      } else if (!answered && tick) {
        clearInterval(tick);
        tick = null;
      }
    });
    inject(DestroyRef).onDestroy(() => {
      if (tick) {
        clearInterval(tick);
      }
    });

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
    if (this.primaryCall() && this.showKeypadDuringCall()) {
      this.phone.sendDtmf(this.primaryCall()!.id, digit as never);
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

  protected retry(): void {
    this.phone.connect();
  }

  /** The caller's display name, falling back to the raw number. */
  protected label(call: CallSession): string {
    return call.displayName?.trim() || call.remote;
  }
}
