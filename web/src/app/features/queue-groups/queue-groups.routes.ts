import { Routes } from '@angular/router';

export const QUEUE_GROUPS_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./queue-groups-list/queue-groups-list').then((m) => m.QueueGroupsListComponent),
  },
];
