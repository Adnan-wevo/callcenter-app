import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';

import { HasPermissionDirective } from '../../../core/authz/has-permission.directive';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { NotificationService } from '../../../shared/services/notification.service';
import { SettingRow, SettingsService } from '../settings.service';

/**
 * The knobs that actually change report behaviour — SLA interval, the
 * short-abandon suppression threshold, agent wrap-up time — resolving open
 * decision D3 (`internal/reports/settings.go`'s own doc comment: "this
 * service cannot read that table yet"). Deliberately narrower than
 * heal-crm's own Settings screen (time_format/display/realtime categories
 * exist there but nothing in this Go service reads them yet — see
 * `migrations/callcenter/004_call_center_settings.sql`'s own comment).
 */
@Component({
  selector: 'app-settings-page',
  standalone: true,
  imports: [PageHeaderComponent, HasPermissionDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './settings-page.html',
})
export class SettingsPageComponent {
  private readonly service = inject(SettingsService);
  private readonly notify = inject(NotificationService);

  protected readonly categories = signal<string[]>([]);
  protected readonly activeCategory = signal('');
  protected readonly rows = signal<SettingRow[]>([]);
  protected readonly values = signal<Record<string, string>>({});
  protected readonly loading = signal(true);
  protected readonly saving = signal(false);

  constructor() {
    this.service.categories().subscribe({
      next: (cats) => {
        this.categories.set(cats);
        if (cats.length > 0) {
          this.switchCategory(cats[0]);
        } else {
          this.loading.set(false);
        }
      },
      error: () => this.loading.set(false),
    });
  }

  protected switchCategory(category: string): void {
    this.activeCategory.set(category);
    this.loading.set(true);
    this.service.list(category).subscribe({
      next: (rows) => {
        this.rows.set(rows);
        this.values.set(Object.fromEntries(rows.map((r) => [r.key, r.value])));
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  protected setValue(key: string, value: string): void {
    this.values.update((v) => ({ ...v, [key]: value }));
  }

  protected save(): void {
    this.saving.set(true);
    this.service.update(this.activeCategory(), this.values()).subscribe({
      next: () => {
        this.saving.set(false);
        this.notify.success('Settings saved.');
        this.switchCategory(this.activeCategory());
      },
      error: () => {
        this.saving.set(false);
        this.notify.error('Could not save the settings.');
      },
    });
  }

  /** Title-cased "short_abandon_threshold" -> "Short abandon threshold". */
  protected label(key: string): string {
    const words = key.split('_');
    return words[0].charAt(0).toUpperCase() + words[0].slice(1) + ' ' + words.slice(1).join(' ');
  }
}
