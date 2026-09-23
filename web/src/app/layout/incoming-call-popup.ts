import { ChangeDetectionStrategy, Component, DestroyRef, effect, inject } from '@angular/core';

import { CallSession } from '../engines/sip/domain/sip-engine';
import { Ringtone } from '../core/softphone/ringtone';
import { SoftphoneService } from '../core/softphone/softphone.service';
import { IconComponent } from '../shared/components/icon/icon';

/**
 * A global "someone is calling" popup — mounted once at the app root
 * (`app.html`), NOT inside `app-softphone-panel`. That placement is the
 * actual fix here: the panel's own ringing card only renders while the
 * panel is open, and its collapsed rail shows the active call but never
 * the ringing one, so an agent with the panel collapsed had literally no
 * visual (and, until `Ringtone`, no audio either) sign of an incoming
 * call. This renders regardless of the panel's open/collapsed state or
 * which route the agent is on.
 */
@Component({
  selector: 'app-incoming-call-popup',
  standalone: true,
  imports: [IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './incoming-call-popup.html',
})
export class IncomingCallPopupComponent {
  protected readonly phone = inject(SoftphoneService);

  private readonly ringtone = new Ringtone();

  constructor() {
    effect(() => {
      if (this.phone.incomingCall()) {
        this.ringtone.start();
      } else {
        this.ringtone.stop();
      }
    });
    inject(DestroyRef).onDestroy(() => this.ringtone.stop());
  }

  protected label(call: CallSession): string {
    return call.displayName?.trim() || call.remote;
  }

  protected queue(call: CallSession): string | null {
    return this.phone.queueForCall(call);
  }

  protected answer(call: CallSession): void {
    this.phone.answer(call.id);
  }

  protected decline(call: CallSession): void {
    this.phone.decline(call.id);
  }
}
