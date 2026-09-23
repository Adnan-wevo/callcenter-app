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
  {
    path: 'answered-calls',
    canActivate: [authGuard, requirePermission('call-center.answered-calls.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () =>
      import('./features/answered-calls/answered-calls.routes').then((m) => m.ANSWERED_CALLS_ROUTES),
  },
  {
    path: 'call-search',
    canActivate: [authGuard, requirePermission('call-center.call-search.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () =>
      import('./features/call-search/call-search.routes').then((m) => m.CALL_SEARCH_ROUTES),
  },
  {
    path: 'distribution',
    canActivate: [authGuard, requirePermission('call-center.distribution.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () =>
      import('./features/distribution/distribution.routes').then((m) => m.DISTRIBUTION_ROUTES),
  },
  {
    path: 'agent-performance',
    canActivate: [authGuard, requirePermission('call-center.agent-performance.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () =>
      import('./features/agent-performance/agent-performance.routes').then(
        (m) => m.AGENT_PERFORMANCE_ROUTES,
      ),
  },
  {
    path: 'realtime-monitor',
    canActivate: [authGuard, requirePermission('call-center.realtime-monitor.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () =>
      import('./features/realtime-monitor/realtime-monitor.routes').then(
        (m) => m.REALTIME_MONITOR_ROUTES,
      ),
  },
  {
    path: 'sip-extensions',
    canActivate: [authGuard, requirePermission('call-center.sip-extensions.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () =>
      import('./features/sip-extensions/sip-extensions.routes').then((m) => m.SIP_EXTENSIONS_ROUTES),
  },
  {
    path: 'user-filters',
    canActivate: [authGuard, requirePermission('call-center.user-filters.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () =>
      import('./features/user-filters/user-filters.routes').then((m) => m.USER_FILTERS_ROUTES),
  },
  {
    path: 'settings',
    canActivate: [authGuard, requirePermission('call-center.settings.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () => import('./features/settings/settings.routes').then((m) => m.SETTINGS_ROUTES),
  },
  {
    path: 'queue-groups',
    canActivate: [authGuard, requirePermission('call-center.queue-groups.index')],
    resolve: { authz: permissionsResolver },
    loadChildren: () =>
      import('./features/queue-groups/queue-groups.routes').then((m) => m.QUEUE_GROUPS_ROUTES),
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
