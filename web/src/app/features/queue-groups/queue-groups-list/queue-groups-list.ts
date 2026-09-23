import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { ApiValidationError } from '../../../core/api/api.types';
import { HasPermissionDirective } from '../../../core/authz/has-permission.directive';
import { IconComponent } from '../../../shared/components/icon/icon';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { PagerComponent } from '../../../shared/components/pager/pager';
import { NotificationService } from '../../../shared/services/notification.service';
import { QueueGroupInput, QueueGroupRow, QueueGroupsService } from '../queue-groups.service';

interface FormState {
  id: string | null;
  name: string;
  description: string;
  is_active: boolean;
  sort_order: number;
  queues: Set<string>;
}

const EMPTY_FORM: FormState = {
  id: null,
  name: '',
  description: '',
  is_active: true,
  sort_order: 0,
  queues: new Set(),
};

/** Named sets of queues an admin can label — organisational metadata only,
 * see the service's own doc comment on why nothing consumes this yet. */
@Component({
  selector: 'app-queue-groups-list',
  standalone: true,
  imports: [IconComponent, PageHeaderComponent, PagerComponent, HasPermissionDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './queue-groups-list.html',
})
export class QueueGroupsListComponent {
  private readonly service = inject(QueueGroupsService);
  private readonly notify = inject(NotificationService);

  protected readonly rows = signal<QueueGroupRow[]>([]);
  protected readonly total = signal(0);
  protected readonly queueOptions = signal<string[]>([]);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  protected readonly search = signal('');
  protected readonly page = signal(1);
  protected readonly pageSize = signal(10);

  protected readonly showForm = signal(false);
  protected readonly form = signal<FormState>({ ...EMPTY_FORM, queues: new Set() });
  protected readonly saving = signal(false);
  protected readonly formError = signal<string | null>(null);

  protected readonly isEditing = computed(() => this.form().id !== null);

  constructor() {
    this.load();
    this.service.queueNames().subscribe({ next: (n) => this.queueOptions.set(n) });
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

  protected openCreate(): void {
    this.form.set({ ...EMPTY_FORM, queues: new Set() });
    this.formError.set(null);
    this.showForm.set(true);
  }

  protected openEdit(row: QueueGroupRow): void {
    this.form.set({
      id: row.id,
      name: row.name,
      description: row.description,
      is_active: row.is_active,
      sort_order: row.sort_order,
      queues: new Set(row.queues),
    });
    this.formError.set(null);
    this.showForm.set(true);
  }

  protected cancelForm(): void {
    this.showForm.set(false);
  }

  protected updateForm(patch: Partial<Omit<FormState, 'queues'>>): void {
    this.form.update((f) => ({ ...f, ...patch }));
  }

  protected toggleQueue(name: string): void {
    this.form.update((f) => {
      const next = new Set(f.queues);
      if (next.has(name)) {
        next.delete(name);
      } else {
        next.add(name);
      }
      return { ...f, queues: next };
    });
  }

  protected submit(): void {
    const f = this.form();
    if (!f.name.trim()) {
      this.formError.set('Name is required.');
      return;
    }

    const body: QueueGroupInput = {
      name: f.name.trim(),
      description: f.description.trim(),
      is_active: f.is_active,
      sort_order: f.sort_order,
      queues: Array.from(f.queues),
    };

    this.saving.set(true);
    this.formError.set(null);
    const onSettled = {
      next: () => {
        this.saving.set(false);
        this.showForm.set(false);
        this.notify.success(f.id ? 'Queue group updated.' : 'Queue group created.');
        this.load();
      },
      error: (err: unknown) => {
        this.saving.set(false);
        this.formError.set(err instanceof ApiValidationError ? err.message : 'Could not save the queue group.');
      },
    };
    if (f.id) {
      this.service.update(f.id, body).subscribe(onSettled);
    } else {
      this.service.create(body).subscribe(onSettled);
    }
  }

  protected remove(row: QueueGroupRow): void {
    if (!confirm(`Delete "${row.name}"? This cannot be undone.`)) {
      return;
    }
    this.service.delete(row.id).subscribe({
      next: () => {
        this.notify.success('Queue group deleted.');
        this.load();
      },
      error: () => this.notify.error('Could not delete the queue group.'),
    });
  }
}
