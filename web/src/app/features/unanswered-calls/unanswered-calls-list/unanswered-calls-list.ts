import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { ApiMeta } from '../../../core/api/api.types';
import { HasPermissionDirective } from '../../../core/authz/has-permission.directive';
import { IconComponent } from '../../../shared/components/icon/icon';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { NotificationService } from '../../../shared/services/notification.service';
import { UnansweredCall, UnansweredCallsService } from '../unanswered-calls.service';

@Component({
  selector: 'app-unanswered-calls-list',
  standalone: true,
  imports: [FormsModule, PageHeaderComponent, IconComponent, HasPermissionDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './unanswered-calls-list.html',
})
export class UnansweredCallsListComponent {
  private readonly service = inject(UnansweredCallsService);
  private readonly notify = inject(NotificationService);

  protected readonly rows = signal<UnansweredCall[]>([]);
  protected readonly meta = signal<ApiMeta | null>(null);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  protected search = '';
  protected readonly searchTerm = signal('');

  /** The row whose callback is awaiting confirmation, or null. */
  protected readonly confirming = signal<UnansweredCall | null>(null);
  protected readonly callingBack = signal(false);

  /**
   * Client-side narrowing over the loaded page.
   *
   * Deliberately local: the server's own search is not wired yet, and
   * filtering what is on screen is honest about that. It must NOT be mistaken
   * for a server filter — it can only ever narrow the current page.
   */
  protected readonly visibleRows = computed(() => {
    const term = this.searchTerm().trim().toLowerCase();
    if (!term) {
      return this.rows();
    }
    return this.rows().filter((row) =>
      [row.caller_id, row.queue_name, row.agent_name, row.event]
        .join(' ')
        .toLowerCase()
        .includes(term),
    );
  });

  constructor() {
    this.load();
  }

  protected load(): void {
    this.loading.set(true);
    this.failed.set(false);

    this.service.list().subscribe({
      next: (page) => {
        this.rows.set(page.rows);
        this.meta.set(page.meta);
        this.loading.set(false);
      },
      error: () => {
        this.failed.set(true);
        this.loading.set(false);
      },
    });
  }

  protected onSearchInput(event: Event): void {
    this.searchTerm.set((event.target as HTMLInputElement).value);
  }

  protected askCallback(row: UnansweredCall): void {
    this.confirming.set(row);
  }

  protected cancelCallback(): void {
    if (!this.callingBack()) {
      this.confirming.set(null);
    }
  }

  protected confirmCallback(): void {
    const row = this.confirming();
    if (!row || this.callingBack()) {
      return;
    }

    this.callingBack.set(true);
    this.service.callback(row.caller_id).subscribe({
      next: () => {
        this.callingBack.set(false);
        this.confirming.set(null);
        // NOTE: this records the attempt only. The Livewire original also
        // dispatches a browser event that makes the softphone dial; an HTTP
        // response cannot, so the dial is still an open question (D7 in
        // docs/extraction-plan.md).
        this.notify.success(`Callback attempt recorded for ${row.caller_id}.`);
      },
      error: () => {
        this.callingBack.set(false);
        this.confirming.set(null);
        // The interceptor already surfaced the failure.
      },
    });
  }

  /** Seconds as m:ss. */
  protected duration(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  }
}
