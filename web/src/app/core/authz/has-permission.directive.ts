import { Directive, TemplateRef, ViewContainerRef, effect, inject, input } from '@angular/core';

import { PermissionStore } from './permission.store';
import { Permission } from './permissions';

/**
 * Renders its host element only when the caller holds the required
 * permission:
 *
 *   <button *hasPermission="'call-center.unanswered-calls.callback'">Call back</button>
 *   <button *hasPermission="['a.index', 'a.index.any']">Open</button>
 *
 * An array is OR-combined, mirroring the route guards and the backend's gates.
 * It reacts to the store, so a permission gained or lost after a resync shows
 * or hides the control without a reload.
 *
 * It is UX only — never a security boundary. The server re-resolves authority
 * on every request and refuses anything the caller lacks, so a control that
 * slips through fails at the API rather than silently succeeding. Hiding it
 * is a courtesy, not the enforcement.
 */
@Directive({ selector: '[hasPermission]' })
export class HasPermissionDirective {
  private readonly tpl = inject(TemplateRef<unknown>);
  private readonly vcr = inject(ViewContainerRef);
  private readonly store = inject(PermissionStore);

  readonly hasPermission = input.required<Permission | Permission[]>();

  private visible = false;

  constructor() {
    effect(() => {
      const value = this.hasPermission();
      const perms = Array.isArray(value) ? value : [value];
      this.render(perms.length > 0 && this.store.canAny(...perms));
    });
  }

  private render(allowed: boolean): void {
    if (allowed && !this.visible) {
      this.vcr.createEmbeddedView(this.tpl);
      this.visible = true;
    } else if (!allowed && this.visible) {
      this.vcr.clear();
      this.visible = false;
    }
  }
}
