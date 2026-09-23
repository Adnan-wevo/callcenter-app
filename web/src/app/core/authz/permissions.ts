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
  'softphone.supervise',
] as const;

export type Permission = (typeof PERMISSIONS)[number];
