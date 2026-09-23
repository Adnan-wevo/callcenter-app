import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { ApiValidationError } from '../../../core/api/api.types';
import { HasPermissionDirective } from '../../../core/authz/has-permission.directive';
import { IconComponent } from '../../../shared/components/icon/icon';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { PagerComponent } from '../../../shared/components/pager/pager';
import { NotificationService } from '../../../shared/services/notification.service';
import {
  REPORT_TYPES,
  ScheduledReportInput,
  ScheduledReportRow,
  ScheduledReportsService,
} from '../scheduled-reports.service';

interface FormState {
  id: string | null;
  name: string;
  destination_email: string;
  reports: Set<string>;
  queues: Set<string>;
  last_days: number;
  cron_day_month: string;
  cron_day_week: string;
  cron_hour: string;
  cron_minute: string;
  is_active: boolean;
}

function emptyForm(): FormState {
  return {
    id: null,
    name: '',
    destination_email: '',
    reports: new Set(),
    queues: new Set(),
    last_days: 1,
    cron_day_month: '*',
    cron_day_week: '*',
    cron_hour: '8',
    cron_minute: '0',
    is_active: true,
  };
}

/**
 * Scheduled-report DEFINITIONS — who gets which report emailed, and on
 * what cron schedule. There is no sender behind this yet (no SMTP, no
 * cron/scheduler process in this Go service — see the service's own doc
 * comment), so the page says so up front rather than implying delivery
 * that doesn't happen.
 */
@Component({
  selector: 'app-scheduled-reports-list',
  standalone: true,
  imports: [IconComponent, PageHeaderComponent, PagerComponent, HasPermissionDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './scheduled-reports-list.html',
})
export class ScheduledReportsListComponent {
  private readonly service = inject(ScheduledReportsService);
  private readonly notify = inject(NotificationService);

  protected readonly reportTypes = REPORT_TYPES;

  protected readonly rows = signal<ScheduledReportRow[]>([]);
  protected readonly total = signal(0);
  protected readonly queueOptions = signal<string[]>([]);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  protected readonly search = signal('');
  protected readonly page = signal(1);
  protected readonly pageSize = signal(10);

  protected readonly showForm = signal(false);
  protected readonly form = signal<FormState>(emptyForm());
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
    this.form.set(emptyForm());
    this.formError.set(null);
    this.showForm.set(true);
  }

  protected openEdit(row: ScheduledReportRow): void {
    this.form.set({
      id: row.id,
      name: row.name,
      destination_email: row.destination_email,
      reports: new Set(row.reports),
      queues: new Set(row.queues),
      last_days: row.last_days,
      cron_day_month: row.cron_day_month,
      cron_day_week: row.cron_day_week,
      cron_hour: row.cron_hour,
      cron_minute: row.cron_minute,
      is_active: row.is_active,
    });
    this.formError.set(null);
    this.showForm.set(true);
  }

  protected cancelForm(): void {
    this.showForm.set(false);
  }

  protected updateForm(patch: Partial<Omit<FormState, 'reports' | 'queues'>>): void {
    this.form.update((f) => ({ ...f, ...patch }));
  }

  protected toggleReport(value: string): void {
    this.form.update((f) => {
      const next = new Set(f.reports);
      next.has(value) ? next.delete(value) : next.add(value);
      return { ...f, reports: next };
    });
  }

  protected toggleQueue(name: string): void {
    this.form.update((f) => {
      const next = new Set(f.queues);
      next.has(name) ? next.delete(name) : next.add(name);
      return { ...f, queues: next };
    });
  }

  protected reportLabel(value: string): string {
    return this.reportTypes.find((r) => r.value === value)?.label ?? value;
  }

  protected submit(): void {
    const f = this.form();
    if (!f.name.trim() || !f.destination_email.trim()) {
      this.formError.set('Name and destination email are required.');
      return;
    }
    if (f.reports.size === 0) {
      this.formError.set('Pick at least one report type.');
      return;
    }

    const body: ScheduledReportInput = {
      name: f.name.trim(),
      destination_email: f.destination_email.trim(),
      reports: Array.from(f.reports),
      queues: Array.from(f.queues),
      last_days: f.last_days,
      cron_day_month: f.cron_day_month.trim() || '*',
      cron_day_week: f.cron_day_week.trim() || '*',
      cron_hour: f.cron_hour.trim(),
      cron_minute: f.cron_minute.trim(),
      is_active: f.is_active,
    };

    this.saving.set(true);
    this.formError.set(null);
    const onSettled = {
      next: () => {
        this.saving.set(false);
        this.showForm.set(false);
        this.notify.success(f.id ? 'Scheduled report updated.' : 'Scheduled report saved.');
        this.load();
      },
      error: (err: unknown) => {
        this.saving.set(false);
        this.formError.set(
          err instanceof ApiValidationError ? err.message : 'Could not save the scheduled report.',
        );
      },
    };
    if (f.id) {
      this.service.update(f.id, body).subscribe(onSettled);
    } else {
      this.service.create(body).subscribe(onSettled);
    }
  }

  protected remove(row: ScheduledReportRow): void {
    if (!confirm(`Delete "${row.name}"? This cannot be undone.`)) {
      return;
    }
    this.service.delete(row.id).subscribe({
      next: () => {
        this.notify.success('Scheduled report deleted.');
        this.load();
      },
      error: () => this.notify.error('Could not delete the scheduled report.'),
    });
  }
}
