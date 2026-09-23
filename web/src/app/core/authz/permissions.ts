/**
 * The CallCenter permission catalogue.
 *
 * These identifiers are the ones the real Laravel policies use
 * (`$this->authorize('call-center.…')` in Modules/CallCenter/app/Livewire),
 * and the Go service names the same constants in internal/auth. Keeping all
 * three identical is what will let authority eventually be served by Laravel
 * without this app changing.
 *
 * It is a union type rather than plain strings so that a typo in a route
 * guard or a `*hasPermission` is a compile error, not a control that silently
 * never appears.
 */
export const PERMISSIONS = [
  'call-center.dashboard.index',
  'call-center.answered-calls.index',
  'call-center.answered-calls.export',
  'call-center.unanswered-calls.index',
  'call-center.unanswered-calls.export',
  'call-center.unanswered-calls.callback',
  'call-center.call-search.index',
  'call-center.call-search.export',
  'call-center.agent-performance.index',
  'call-center.distribution.index',
  'call-center.sip-extensions.index',
  'call-center.sip-extensions.store',
  'call-center.sip-extensions.update',
  'call-center.sip-extensions.destroy',
  'call-center.user-filters.index',
  'call-center.user-filters.edit',
  'call-center.user-filters.update',
  'call-center.user-filters.destroy',
  'call-center.realtime-monitor.index',
  'call-center.realtime-monitor.actions',
  'call-center.settings.index',
  'call-center.settings.update',
  'call-center.queue-groups.index',
  'call-center.queue-groups.store',
  'call-center.queue-groups.update',
  'call-center.queue-groups.destroy',
  'call-center.scheduled-reports.index',
  'call-center.scheduled-reports.store',
  'call-center.scheduled-reports.update',
  'call-center.scheduled-reports.destroy',
  'softphone.supervise',
] as const;

export type Permission = (typeof PERMISSIONS)[number];
