import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from '@angular/core';

import { SoftphoneService } from '../../core/softphone/softphone.service';
import { IconComponent } from '../../shared/components/icon/icon';
import { PagerComponent } from '../../shared/components/pager/pager';

/**
 * The agent's own past calls. Pagination is server-side (this table can
 * hold a long career's worth of rows, unlike Queue/Contacts which are a
 * live snapshot) — page/pageSize drive a fetch rather than slicing an
 * in-memory list. Search and the status filter apply to whatever page is
 * currently loaded, not the whole history — a deliberately smaller promise
 * than heal-crm's fully-in-memory `filteredHistory`, which only works there
 * because its history array is never paged out from under it.
 */
@Component({
  selector: 'app-history-tab',
  standalone: true,
  imports: [IconComponent, PagerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './history-tab.html',
})
export class HistoryTabComponent {
  protected readonly phone = inject(SoftphoneService);

  protected readonly search = signal('');
  protected readonly statusFilter = signal('');
  protected readonly page = signal(1);
  protected readonly pageSize = signal(25);

  protected readonly total = computed(() => this.phone.historyMeta()?.total ?? 0);

  protected readonly filtered = computed(() => {
    const q = this.search().toLowerCase().trim();
    const status = this.statusFilter();
    let list = this.phone.historyEntries();
    if (status) {
      list = list.filter((h) => h.status === status);
    }
    if (q) {
      list = list.filter(
        (h) => h.caller_name.toLowerCase().includes(q) || h.caller_id.toLowerCase().includes(q),
      );
    }
    return list;
  });

  constructor() {
    effect(() => {
      this.phone.loadHistory(this.page(), this.pageSize());
    });
  }

  protected duration(seconds: number): string {
    if (!seconds) {
      return '—';
    }
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  }

  protected time(iso: string | null): string {
    if (!iso) {
      return '';
    }
    return new Date(iso).toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }
}
