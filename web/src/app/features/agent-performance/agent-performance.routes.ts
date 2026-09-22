import { Routes } from '@angular/router';

export const AGENT_PERFORMANCE_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./agent-performance-page/agent-performance-page').then(
        (m) => m.AgentPerformancePageComponent,
      ),
  },
];
