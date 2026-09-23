import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { SoftphoneService } from '../../core/softphone/softphone.service';
import { IconComponent } from '../../shared/components/icon/icon';
import { PagerComponent } from '../../shared/components/pager/pager';

/** pbx-worker's own agent.status strings (confirmed against the live
 * staging worker: 'unavailable', 'on_call', plus these device-state-ish
 * ones some deployments send) — loosely matched, anything else falls back
 * to 'offline' grey rather than guessing. */
const ONLINE_STATUSES = new Set(['online', 'not_inuse', 'available', 'idle', 'ready']);
const BUSY_STATUSES = new Set(['busy', 'inuse', 'in_use', 'ringing', 'onhold', 'on_hold', 'oncall', 'on_call']);

@Component({
  selector: 'app-contacts-tab',
  standalone: true,
  imports: [IconComponent, PagerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './contacts-tab.html',
})
export class ContactsTabComponent {
  protected readonly phone = inject(SoftphoneService);

  protected readonly search = signal('');
  protected readonly statusFilter = signal('');
  protected readonly page = signal(1);
  protected readonly pageSize = signal(25);

  protected readonly canDial = computed(() => this.phone.isConnected() && this.phone.activeCalls().length === 0);

  protected readonly filtered = computed(() => {
    const q = this.search().toLowerCase().trim();
    const status = this.statusFilter();
    let list = this.phone.contacts();
    if (status) {
      list = list.filter((c) => this.tone(c.status, c.onCall) === status);
    }
    if (q) {
      list = list.filter((c) => c.extension.toLowerCase().includes(q));
    }
    return list;
  });

  protected readonly paginated = computed(() => {
    const start = (this.page() - 1) * this.pageSize();
    return this.filtered().slice(start, start + this.pageSize());
  });

  protected tone(status: string, onCall: boolean): 'online' | 'busy' | 'offline' {
    if (onCall || BUSY_STATUSES.has(status.toLowerCase())) {
      return 'busy';
    }
    if (ONLINE_STATUSES.has(status.toLowerCase())) {
      return 'online';
    }
    return 'offline';
  }

  protected call(extension: string): void {
    this.phone.dial(extension);
  }
}
