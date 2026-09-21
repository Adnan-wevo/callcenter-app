import { inject } from '@angular/core';
import { CanActivateFn, ResolveFn, Router } from '@angular/router';
import { map } from 'rxjs';

import { PermissionState, PermissionStore } from './permission.store';
import { Permission } from './permissions';

/**
 * Admits the route only when the caller holds AT LEAST ONE of `perms`.
 *
 * It first ensures authority is loaded, so a deep link into a gated route on
 * a cold page load waits for the real resolved set rather than deciding
 * against an empty one.
 *
 * The two failure paths lead to different places on purpose:
 *
 *   - A MISSING PERMISSION goes to /forbidden. The caller is signed in and
 *     their authority loaded; they simply may not open this, and a named page
 *     says so instead of silently bouncing them to the home page.
 *   - A FAILED authority LOAD goes to /dashboard, where the shell renders the
 *     retry screen. Nothing was refused — the authority could not be read at
 *     all — so it must not claim to be a 403. It never admits the route.
 *
 * Multiple permissions are OR-combined, mirroring the backend.
 */
export function requirePermission(...perms: Permission[]): CanActivateFn {
  return () => {
    const store = inject(PermissionStore);
    const router = inject(Router);

    return store.ensureLoaded().pipe(
      map((state: PermissionState) => {
        if (state !== 'ready') {
          return router.createUrlTree(['/dashboard']);
        }
        return store.canAny(...perms) ? true : router.createUrlTree(['/forbidden']);
      }),
    );
  };
}

/**
 * Triggers (and waits for) the authority bootstrap before a route activates.
 *
 * Attaching it to every authed route guarantees the store is populated before
 * any authed view renders — including ungated ones — so the shell shows the
 * ready app, the spinner or the error screen from accurate state rather than
 * flashing an inert shell. It resolves on both terminal states, so an error
 * does not wedge navigation.
 */
export const permissionsResolver: ResolveFn<PermissionState> = () =>
  inject(PermissionStore).ensureLoaded();
