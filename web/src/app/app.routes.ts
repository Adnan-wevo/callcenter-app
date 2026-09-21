import { Routes } from '@angular/router';

import { authGuard, guestGuard } from './core/auth/auth.guard';
import { permissionsResolver, requirePermission } from './core/authz/permission.guard';

/**
 * Every authed route carries authGuard (a valid token) AND the
 * permissionsResolver (which awaits the authority bootstrap so the view
 * renders against real, resolved authority rather than an empty set).
 * Routes needing a capability add requirePermission naming the permissions
 * that admit them, OR-combined.
 *
 * There is no permissive default anywhere: absence of a permission is a
 * denial, and a route with no requirePermission is one any signed-in caller
 * may open.
 */
export const routes: Routes = [
  {
    // guestGuard bounces an already-signed-in caller to their home, so the
    // login form is never shown to somebody who does not need it.
    path: 'login',
    canActivate: [guestGuard],
    loadComponent: () => import('./features/login/login').then((m) => m.LoginComponent),
  },
  {
    // Carries NO guard, so it is reachable signed in or out.
    path: 'forbidden',
    data: { kind: 'forbidden' },
    loadComponent: () => import('./features/errors/error-page').then((m) => m.ErrorPageComponent),
  },
  {
    path: 'dashboard',
    canActivate: [authGuard, requirePermission('call-center.dashboard.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () => import('./features/dashboard/dashboard.routes').then((m) => m.DASHBOARD_ROUTES),
  },
  {
    path: 'unanswered-calls',
    canActivate: [authGuard, requirePermission('call-center.unanswered-calls.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () =>
      import('./features/unanswered-calls/unanswered-calls.routes').then(
        (m) => m.UNANSWERED_CALLS_ROUTES,
      ),
  },
  { path: '', pathMatch: 'full', redirectTo: 'dashboard' },
  {
    // An unknown URL is a 404, not a silent bounce to the dashboard: the
    // redirect hides a dead link by pretending it was the home page.
    path: '**',
    data: { kind: 'not-found' },
    loadComponent: () => import('./features/errors/error-page').then((m) => m.ErrorPageComponent),
  },
];
