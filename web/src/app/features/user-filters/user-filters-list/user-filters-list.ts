import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';

import { HasPermissionDirective } from '../../../core/authz/has-permission.directive';
import { IconComponent } from '../../../shared/components/icon/icon';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { PagerComponent } from '../../../shared/components/pager/pager';
import { NotificationService } from '../../../shared/services/notification.service';
import { UserFilterRow, UserFiltersService } from '../user-filters.service';

/**
 * Row-level security admin: which queues/agents each user is restricted to
 * seeing in the report screens (`auth.User.AllowedQueues/AllowedAgents`,
 * already enforced server-side wherever a report reads them — this screen
 * is the missing piece that lets an admin actually SET them, mirroring
 * `Modules/CallCenter/app/Livewire/UserFilters/Index.php`).
 *
 * Empty = unrestricted (sees everything) — not "restricted to nothing".
 */
@Component({
  selector: 'app-user-filters-list',
  standalone: true,
  imports: [IconComponent, PageHeaderComponent, PagerComponent, HasPermissionDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './user-filters-list.html',
})
export class UserFiltersListComponent {
  private readonly service = inject(UserFiltersService);
  private readonly notify = inject(NotificationService);

  protected readonly rows = signal<UserFilterRow[]>([]);
  protected readonly total = signal(0);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  protected readonly search = signal('');
  protected readonly page = signal(1);
  protected readonly pageSize = signal(10);

  protected readonly queueOptions = signal<string[]>([]);
  protected readonly agentOptions = signal<string[]>([]);

  protected readonly editingId = signal<string | null>(null);
  protected readonly formQueues = signal<Set<string>>(new Set());
  protected readonly formAgents = signal<Set<string>>(new Set());
  protected readonly saving = signal(false);

  constructor() {
    this.load();
    this.service.queueNames().subscribe({ next: (n) => this.queueOptions.set(n) });
    this.service.agentNames().subscribe({ next: (n) => this.agentOptions.set(n) });
  }

  protected load(): void {
    this.loading.set(true);
    this.failed.set(false);
    this.service
      .list({ search: this.search(), page: this.page(), per_page: this.pageSize() })
      .subscribe({
        next: (result) => {
          this.rows.set(result.rows);
          this.total.set(result.meta.total);
          this.loading.set(false);
        },
        error: () => {
          this.loading.set(false);
          this.failed.set(true);
        },
      });
  }

  protected onSearchInput(event: Event): void {
    this.search.set((event.target as HTMLInputElement).value);
    this.page.set(1);
    this.load();
  }

  protected onPageChange(page: number): void {
    this.page.set(page);
    this.load();
  }

  protected onPageSizeChange(size: number): void {
    this.pageSize.set(size);
    this.page.set(1);
    this.load();
  }

  protected openEdit(row: UserFilterRow): void {
    this.editingId.set(row.id);
    this.formQueues.set(new Set(row.allowed_queues));
    this.formAgents.set(new Set(row.allowed_agents));
  }

  protected cancelEdit(): void {
    this.editingId.set(null);
  }

  protected toggle(set: 'queues' | 'agents', value: string): void {
    const signalRef = set === 'queues' ? this.formQueues : this.formAgents;
    signalRef.update((current) => {
      const next = new Set(current);
      if (next.has(value)) {
        next.delete(value);
      } else {
        next.add(value);
      }
      return next;
    });
  }

  protected save(row: UserFilterRow): void {
    this.saving.set(true);
    this.service
      .update(row.id, Array.from(this.formQueues()), Array.from(this.formAgents()))
      .subscribe({
        next: () => {
          this.saving.set(false);
          this.editingId.set(null);
          this.notify.success('Filters saved.');
          this.load();
        },
        error: () => {
          this.saving.set(false);
          this.notify.error('Could not save the filters.');
        },
      });
  }

  protected clear(row: UserFilterRow): void {
    if (!confirm(`Remove all filters for ${row.username}? They'll see every queue and agent again.`)) {
      return;
    }
    this.service.clear(row.id).subscribe({
      next: () => {
        this.notify.success('Filters cleared.');
        this.load();
      },
      error: () => this.notify.error('Could not clear the filters.'),
    });
  }
}
