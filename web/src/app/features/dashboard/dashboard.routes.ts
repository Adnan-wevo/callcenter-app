import { Routes } from '@angular/router';

/** One route, because the dashboard is one screen. */
export const DASHBOARD_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () => import('./dashboard-page/dashboard-page').then((m) => m.DashboardPageComponent),
  },
];
