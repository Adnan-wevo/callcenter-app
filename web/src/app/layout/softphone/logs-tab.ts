import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { DebugLevel } from '../../core/softphone/softphone.types';
import { SoftphoneService } from '../../core/softphone/softphone.service';
import { IconComponent } from '../../shared/components/icon/icon';
import { PagerComponent } from '../../shared/components/pager/pager';

/**
 * A running record of what THIS session's softphone did — registration
 * transitions, call events, queue/supervisor command results — never sent
 * to the server. This is the client-only diagnostic log heal-crm's own
 * "Logs" tab is (its `debugLogs`, purely in-memory, cleared on reload —
 * see `default.blade.php`'s own comment on it); it is NOT this agent's
 * call history, which is what the History tab is for.
 */
@Component({
  selector: 'app-logs-tab',
  standalone: true,
  imports: [IconComponent, PagerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './logs-tab.html',
})
export class LogsTabComponent {
  protected readonly phone = inject(SoftphoneService);

  protected readonly search = signal('');
  protected readonly levelFilter = signal<DebugLevel | ''>('');
  protected readonly page = signal(1);
  protected readonly pageSize = signal(25);

  protected readonly filtered = computed(() => {
    const q = this.search().toLowerCase().trim();
    const level = this.levelFilter();
    let list = this.phone.logs();
    if (level) {
      list = list.filter((l) => l.level === level);
    }
    if (q) {
      list = list.filter((l) => l.message.toLowerCase().includes(q));
    }
    return list;
  });

  protected readonly paginated = computed(() => {
    const start = (this.page() - 1) * this.pageSize();
    return this.filtered().slice(start, start + this.pageSize());
  });

  protected time(at: Date): string {
    return at.toLocaleTimeString(undefined, { hour12: false });
  }
}
