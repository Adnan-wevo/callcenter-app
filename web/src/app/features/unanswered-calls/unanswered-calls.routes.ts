import { Routes } from '@angular/router';

/**
 * One route. The call-detail view is a drawer over the list rather than a
 * route of its own, for the reason the console's own list screens give: the
 * context the user is working in should not disappear to look at one row.
 */
export const UNANSWERED_CALLS_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./unanswered-calls-list/unanswered-calls-list').then(
        (m) => m.UnansweredCallsListComponent,
      ),
  },
];
