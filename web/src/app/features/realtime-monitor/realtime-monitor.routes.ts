import { Routes } from '@angular/router';

export const REALTIME_MONITOR_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./realtime-monitor-page/realtime-monitor-page').then(
        (m) => m.RealtimeMonitorPageComponent,
      ),
  },
];
