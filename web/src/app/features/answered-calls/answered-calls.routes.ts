import { Routes } from '@angular/router';

export const ANSWERED_CALLS_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./answered-calls-list/answered-calls-list').then((m) => m.AnsweredCallsListComponent),
  },
];
