import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { ApiValidationError } from '../../../core/api/api.types';
import { HasPermissionDirective } from '../../../core/authz/has-permission.directive';
import { IconComponent } from '../../../shared/components/icon/icon';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { PagerComponent } from '../../../shared/components/pager/pager';
import { NotificationService } from '../../../shared/services/notification.service';
import { UserInput, UserRow, UsersService } from '../users.service';

interface FormState {
  id: string | null;
  name: string;
  email: string;
  password: string;
  role: string;
}

const EMPTY_FORM: FormState = { id: null, name: '', email: '', password: '', role: '' };

/** The shortest password v3 accepts. Checked here so the form says so
 *  inline rather than the caller learning it from a rejected save. */
const MIN_PASSWORD = 6;

/**
 * Who may sign in, and as what.
 *
 * Backed by pbx-worker v3's directory rather than a table of this service's
 * own, because that is the directory sign-ins already fall through to — so
 * an account created here works immediately, and there is no second user
 * list to keep in step. v3 is API-only, which is why the screen lives here.
 *
 * The role is the consequential field: it is what this service derives a
 * user's authority from (see auth.permissionsForRoles), so the options come
 * from v3 rather than being hardcoded, and a role is required on create.
 */
@Component({
  selector: 'app-users-list',
  standalone: true,
  imports: [IconComponent, PageHeaderComponent, PagerComponent, HasPermissionDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './users-list.html',
})
export class UsersListComponent {
  private readonly service = inject(UsersService);
  private readonly notify = inject(NotificationService);

  protected readonly rows = signal<UserRow[]>([]);
  protected readonly total = signal(0);
  protected readonly roleOptions = signal<string[]>([]);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  protected readonly search = signal('');
  protected readonly page = signal(1);
  protected readonly pageSize = signal(10);

  protected readonly showForm = signal(false);
  protected readonly form = signal<FormState>({ ...EMPTY_FORM });
  protected readonly saving = signal(false);
  protected readonly formError = signal<string | null>(null);

  protected readonly isEditing = computed(() => this.form().id !== null);

  constructor() {
    this.load();
    this.service.roles().subscribe({ next: (r) => this.roleOptions.set(r) });
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

  protected openEdit(row: UserRow): void {
    this.form.set({
      id: row.id,
      name: row.name,
      email: row.email,
      // Never populated from a row: this app never sees an existing
      // password, and a blank field on edit means "leave it alone".
      password: '',
      role: row.roles[0] ?? '',
    });
    this.formError.set(null);
    this.showForm.set(true);
  }

  protected cancelForm(): void {
    this.showForm.set(false);
  }

  protected updateForm(patch: Partial<FormState>): void {
    this.form.update((f) => ({ ...f, ...patch }));
  }

  protected submit(): void {
    const f = this.form();
    const name = f.name.trim();
    const email = f.email.trim();

    if (!name || !email) {
      this.formError.set('Name and email are required.');
      return;
    }
    if (!f.role) {
      // Enforced rather than defaulted: a silent default decides what
      // somebody can do in this system, which should be a deliberate choice.
      this.formError.set('Pick a role — it decides what this user can do.');
      return;
    }
    if (!f.id && f.password.length < MIN_PASSWORD) {
      this.formError.set(`Password must be at least ${MIN_PASSWORD} characters.`);
      return;
    }

    const body: UserInput = f.id
      ? { name, email, role: f.role }
      : { name, email, role: f.role, password: f.password };

    this.saving.set(true);
    this.formError.set(null);
    const onSettled = {
      next: () => {
        this.saving.set(false);
        this.showForm.set(false);
        this.notify.success(f.id ? 'User updated.' : 'User created.');
        this.load();
      },
      error: (err: unknown) => {
        this.saving.set(false);
        this.formError.set(
          err instanceof ApiValidationError ? err.message : 'Could not save the user.',
        );
      },
    };
    if (f.id) {
      this.service.update(f.id, body).subscribe(onSettled);
    } else {
      this.service.create(body).subscribe(onSettled);
    }
  }

  protected remove(row: UserRow): void {
    const label = row.name || row.email;
    if (!confirm(`Delete "${label}"? They will no longer be able to sign in. This cannot be undone.`)) {
      return;
    }
    this.service.delete(row.id).subscribe({
      next: () => {
        this.notify.success('User deleted.');
        this.load();
      },
      error: () => this.notify.error('Could not delete that user.'),
    });
  }
}
