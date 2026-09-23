import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';

import { IconComponent } from '../icon/icon';

/**
 * First/prev/page-of-total/next/last, plus a page-size select — the same
 * pager heal-crm's softphone panel reuses across its Queue/Contacts/History
 * tabs (`Modules/SoftPhone/resources/views/livewire/partials/pager.blade.php`).
 * One component here for the same reason: three lists, one pager.
 *
 * Page and pageSize are `model()`s (two-way) so a parent can just
 * `[(page)]`/`[(pageSize)]` bind them straight into whatever slices its own
 * list — this component owns no list state itself.
 */
@Component({
  selector: 'app-pager',
  standalone: true,
  imports: [IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="flex items-center justify-between gap-2 border-t pt-2 text-xs">
      <div class="flex items-center gap-0.5">
        <button
          type="button"
          class="hover:bg-accent disabled:opacity-40 rounded p-1 transition-colors"
          [disabled]="page() <= 1"
          title="First page"
          aria-label="First page"
          (click)="page.set(1)"
        >
          <app-icon name="chevrons-left" [size]="14" />
        </button>
        <button
          type="button"
          class="hover:bg-accent disabled:opacity-40 rounded p-1 transition-colors"
          [disabled]="page() <= 1"
          title="Previous page"
          aria-label="Previous page"
          (click)="page.set(page() - 1)"
        >
          <app-icon name="chevron-left" [size]="14" />
        </button>
        <span class="text-muted-foreground px-1 tabular-nums">
          <strong class="text-foreground">{{ page() }}</strong> / {{ totalPages() }}
        </span>
        <button
          type="button"
          class="hover:bg-accent disabled:opacity-40 rounded p-1 transition-colors"
          [disabled]="page() >= totalPages()"
          title="Next page"
          aria-label="Next page"
          (click)="page.set(page() + 1)"
        >
          <app-icon name="chevron-right" [size]="14" />
        </button>
        <button
          type="button"
          class="hover:bg-accent disabled:opacity-40 rounded p-1 transition-colors"
          [disabled]="page() >= totalPages()"
          title="Last page"
          aria-label="Last page"
          (click)="page.set(totalPages())"
        >
          <app-icon name="chevrons-right" [size]="14" />
        </button>
      </div>

      <select
        class="input h-6 w-auto py-0 text-xs"
        aria-label="Page size"
        [value]="pageSize()"
        (change)="pageSize.set(+$any($event.target).value)"
      >
        <option [value]="10">10</option>
        <option [value]="25">25</option>
        <option [value]="50">50</option>
        <option [value]="100">100</option>
      </select>
    </div>
  `,
})
export class PagerComponent {
  readonly page = model.required<number>();
  readonly pageSize = model.required<number>();
  readonly total = input.required<number>();

  protected readonly totalPages = computed(() => Math.max(1, Math.ceil(this.total() / this.pageSize())));
}
