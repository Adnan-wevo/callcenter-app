import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';

import { CallSession } from '../engines/sip/domain/sip-engine';
import { SoftphoneService } from '../core/softphone/softphone.service';
import { IconComponent } from '../shared/components/icon/icon';

/**
 * The "screen pop" — a right-side panel that appears once a call is
 * actually connected (`SoftphoneService.answeredCall`), showing who it is
 * and where it came from as a visual card: avatar, number, direction,
 * which of the agent's own extensions took it, the queue it arrived
 * through, and a live timer. Deliberately NOT a debug/JSON view (that's
 * what the softphone panel's own Logs tab is for) and deliberately NOT an
 * editable wrap-up/disposition form — this app is scoped to the call
 * center itself, not a CRM (see the parity audit's "3. Wrap-up" row,
 * which this does not attempt to close).
 *
 * Purely informational: mute/hold/hangup/keypad stay in `app-softphone-panel`
 * exactly as they are today. This is a read-only sibling, not a redesign.
 */
@Component({
  selector: 'app-call-detail-panel',
  standalone: true,
  imports: [IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './call-detail-panel.html',
})
export class CallDetailPanelComponent {
  protected readonly phone = inject(SoftphoneService);

  /** Up to two initials from the caller's name, or a leading digit/'#' from
   * a bare number when no name is known — always something, never blank. */
  protected readonly initials = computed(() => {
    const call = this.phone.answeredCall();
    const name = call?.displayName?.trim();
    if (name) {
      return name
        .split(/\s+/)
        .slice(0, 2)
        .map((part) => part[0]?.toUpperCase())
        .join('');
    }
    return call?.remote ? '#' : '';
  });

  protected label(call: CallSession): string {
    return call.displayName?.trim() || call.remote;
  }
}
