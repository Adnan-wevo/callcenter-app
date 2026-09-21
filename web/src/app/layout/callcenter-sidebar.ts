import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';

import { IconComponent } from '../shared/components/icon/icon';
import { NAV_AUTHORITY, NavBrand, NavGroup, Reveals } from './nav';

/**
 * The application shell: the brand header, the nav, and the slots a page
 * fills in.
 *
 * Entries are resolved against NAV_AUTHORITY rather than against the
 * permission store directly, so the same shell could later render a second
 * surface with a different catalogue without knowing anything about it.
 *
 * A hidden entry is NOT the enforcement — the route guard and the server both
 * re-check. This only decides what a user is offered.
 */
@Component({
  selector: 'app-sidebar',
  standalone: true,
  imports: [RouterLink, RouterLinkActive, IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <aside
      class="bg-sidebar text-sidebar-foreground flex h-full w-64 shrink-0 flex-col border-r"
    >
      <!-- The header names the surface AND is the way home. The logo tile is
           8x8 rounded-lg on the brand colour, matching the console's own. -->
      <a
        [routerLink]="brand().home"
        class="hover:bg-sidebar-accent flex items-center gap-2 border-b p-3 transition-colors"
      >
        <div
          class="bg-brand text-brand-foreground flex aspect-square size-8 shrink-0 items-center justify-center rounded-lg"
        >
          <app-icon name="phone-call" [size]="18" />
        </div>
        <div class="grid min-w-0 flex-1 text-left leading-tight">
          <span class="truncate text-sm font-semibold">{{ brand().title }}</span>
          <span class="text-muted-foreground truncate text-xs">{{ brand().subtitle }}</span>
        </div>
      </a>

      <nav class="flex-1 overflow-y-auto p-2">
        @for (group of visibleGroups(); track group.label) {
          <div class="mb-4">
            <div class="text-muted-foreground px-2 py-1.5 text-xs font-medium">
              {{ group.label }}
            </div>
            <ul class="grid gap-0.5">
              @for (item of group.items; track item.route) {
                <li>
                  <a
                    [routerLink]="item.route"
                    routerLinkActive="bg-sidebar-accent text-sidebar-accent-foreground font-medium"
                    class="hover:bg-sidebar-accent flex items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors"
                  >
                    <app-icon [name]="item.icon" />
                    <span class="truncate">{{ item.label }}</span>
                  </a>
                </li>
              }
            </ul>
          </div>
        }
      </nav>

      <ng-content select="[footer]" />
    </aside>
  `,
})
export class SidebarComponent {
  private readonly authority = inject(NAV_AUTHORITY);

  readonly brand = input.required<NavBrand>();
  readonly groups = input.required<NavGroup[]>();

  /**
   * Groups with their unreachable entries removed, and any group left empty
   * dropped entirely — a heading over nothing reads as a broken section
   * rather than as one the caller cannot use.
   */
  protected readonly visibleGroups = computed<NavGroup[]>(() =>
    this.groups()
      .map((group) => ({
        ...group,
        items: group.items.filter((item) => this.reveals(item.permission)),
      }))
      .filter((group) => group.items.length > 0),
  );

  private reveals(permission: Reveals): boolean {
    if (permission === null) {
      return true;
    }
    const names = Array.isArray(permission) ? permission : [permission];
    return this.authority.canAny(...names);
  }
}
