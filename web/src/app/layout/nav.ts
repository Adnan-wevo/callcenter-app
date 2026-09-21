import { InjectionToken } from '@angular/core';

/**
 * The nav vocabulary the shell renders.
 *
 * # Why permission is a plain string here
 *
 * This file is the SHELL's contract, and a shell has to be able to render a
 * surface whose catalogue it does not know. The type is therefore widened at
 * this seam and narrowed again where a surface DECLARES its nav:
 * callcenter-nav.ts types its literals against the generated `Permission`
 * union, so a typo there is still a compile error, and the assignment to
 * `NavGroup[]` widens it on the way out.
 */
export type Reveals = string | string[] | null;

export interface NavItem {
  label: string;
  route: string;
  icon: string;
  /** null means always visible; an array is OR-combined. */
  permission: Reveals;
}

export interface NavGroup {
  label: string;
  items: NavItem[];
}

/** The branding shown in the sidebar header. */
export interface NavBrand {
  title: string;
  subtitle: string;
  /** Where the logo tile navigates. */
  home: string;
}

/**
 * What the sidebar asks to decide whether an entry is offered.
 *
 * Deliberately the narrowest thing the shell needs, so a second authority
 * source (a tenant store, say) can satisfy it without the shell knowing.
 *
 * A visible entry is never a grant: the route guard and the server both
 * re-check. This only decides what a user is OFFERED.
 */
export interface NavAuthority {
  canAny(...names: string[]): boolean;
}

/**
 * The authority the sidebar resolves entries against.
 *
 * Provided at the app root as the PermissionStore. It is a token rather than
 * an `@Input` because it is a service, and because a shell that forgot to
 * pass one would otherwise silently fall back to the wrong authority.
 */
export const NAV_AUTHORITY = new InjectionToken<NavAuthority>('NAV_AUTHORITY');
