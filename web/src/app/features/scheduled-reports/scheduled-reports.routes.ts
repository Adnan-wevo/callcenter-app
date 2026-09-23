import { Routes } from '@angular/router';

export const SCHEDULED_REPORTS_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./scheduled-reports-list/scheduled-reports-list').then(
        (m) => m.ScheduledReportsListComponent,
      ),
  },
];
