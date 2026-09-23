import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { ApiValidationError } from '../../../core/api/api.types';
import { HasPermissionDirective } from '../../../core/authz/has-permission.directive';
import { IconComponent } from '../../../shared/components/icon/icon';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { PagerComponent } from '../../../shared/components/pager/pager';
import { NotificationService } from '../../../shared/services/notification.service';
import {
  SipExtensionInput,
  SipExtensionRow,
  SipExtensionsService,
  SipUser,
} from '../sip-extensions.service';

interface FormState {
  id: string | null;
  user_id: string;
  extension: string;
  sip_username: string;
  sip_password: string;
  display_name: string;
  queues: string;
  is_supervisor: boolean;
  is_default: boolean;
}

const EMPTY_FORM: FormState = {
  id: null,
  user_id: '',
  extension: '',
  sip_username: '',
  sip_password: '',
  display_name: '',
  queues: '',
  is_supervisor: false,
  is_default: false,
};

/**
 * Who may register as which extension — the admin CRUD the backend has
 * carried since `internal/handlers/sipextensions.go` was ported, with no
 * screen in front of it until now.
 *
 * The list endpoint never returns `sip_username` (see
 * `SipExtensionRow`'s own doc comment) and the update endpoint requires it
 * anyway (same struct, same `binding:"required"` tag, as create) — so
 * editing a row means re-entering its SIP username, not just its changed
 * fields. That's the backend's actual contract, not a gap introduced here.
 */
@Component({
  selector: 'app-sip-extensions-list',
  standalone: true,
  imports: [IconComponent, PageHeaderComponent, PagerComponent, HasPermissionDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './sip-extensions-list.html',
})
export class SipExtensionsListComponent {
  private readonly service = inject(SipExtensionsService);
  private readonly notify = inject(NotificationService);

  protected readonly rows = signal<SipExtensionRow[]>([]);
  protected readonly total = signal(0);
  protected readonly users = signal<SipUser[]>([]);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  protected readonly search = signal('');
  protected readonly page = signal(1);
  protected readonly pageSize = signal(15);

  protected readonly showForm = signal(false);
  protected readonly form = signal<FormState>({ ...EMPTY_FORM });
  protected readonly saving = signal(false);
  protected readonly formError = signal<string | null>(null);

  protected readonly isEditing = computed(() => this.form().id !== null);

  protected readonly usernameFor = computed(() => {
    const map = new Map(this.users().map((u) => [u.id, u.username]));
    return (userId: string) => map.get(userId) ?? userId;
  });

  constructor() {
    this.load();
    this.service.users().subscribe({ next: (u) => this.users.set(u) });
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
    this.form.set({ ...EMPTY_FORM });
    this.formError.set(null);
    this.showForm.set(true);
  }

  protected openEdit(row: SipExtensionRow): void {
    this.form.set({
      id: row.id,
      user_id: row.user_id,
      extension: row.extension,
      sip_username: '',
      sip_password: '',
      display_name: row.display_name,
      queues: row.queues.join('|'),
      is_supervisor: row.is_supervisor,
      is_default: row.is_default,
    });
    this.formError.set(null);
    this.showForm.set(true);
  }

  protected cancelForm(): void {
    this.showForm.set(false);
    this.formError.set(null);
  }

  protected updateForm(patch: Partial<FormState>): void {
    this.form.update((f) => ({ ...f, ...patch }));
  }

  protected submit(): void {
    const f = this.form();
    if (!f.extension.trim() || !f.sip_username.trim()) {
      this.formError.set('Extension and SIP username are required.');
      return;
    }
    if (!f.id && !f.user_id) {
      this.formError.set('Choose which user this extension belongs to.');
      return;
    }
    if (!f.id && f.sip_password.length < 8) {
      this.formError.set('SIP password must be at least 8 characters.');
      return;
    }

    const body: SipExtensionInput = {
      extension: f.extension.trim(),
      sip_username: f.sip_username.trim(),
      display_name: f.display_name.trim(),
      queues: f.queues.trim(),
      is_supervisor: f.is_supervisor,
      is_default: f.is_default,
    };
    if (f.sip_password) {
      body.sip_password = f.sip_password;
    }
    if (!f.id) {
      body.user_id = f.user_id;
    }

    this.saving.set(true);
    this.formError.set(null);
    const onSettled = {
      next: () => {
        this.saving.set(false);
        this.showForm.set(false);
        this.notify.success(f.id ? 'Extension updated.' : 'Extension created.');
        this.load();
      },
      error: (err: unknown) => {
        this.saving.set(false);
        if (err instanceof ApiValidationError) {
          this.formError.set(err.message);
        } else {
          this.formError.set('Could not save the extension.');
        }
      },
    };
    if (f.id) {
      this.service.update(f.id, body).subscribe(onSettled);
    } else {
      this.service.create(body).subscribe(onSettled);
    }
  }

  protected remove(row: SipExtensionRow): void {
    if (!confirm(`Delete extension ${row.extension}? This cannot be undone.`)) {
      return;
    }
    this.service.delete(row.id).subscribe({
      next: () => {
        this.notify.success('Extension deleted.');
        this.load();
      },
      error: () => this.notify.error('Could not delete the extension.'),
    });
  }
}
