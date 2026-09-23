import { Permission } from '../core/authz/permissions';
import { NavBrand, NavGroup } from './nav';

/**
 * The Call Center surface's navigation and branding.
 *
 * # Why it is declared with its own narrower types
 *
 * The shell's NavGroup widens `permission` to a plain string so it can also
 * render a surface whose catalogue has no union type. Declaring this file
 * against the aliases below keeps the generated `Permission` union enforced
 * on every literal, so a renamed or mistyped identifier is a compile error
 * rather than a nav entry that silently never appears. The assignment to
 * NavGroup[] at the bottom is what widens it.
 */
type CallCenterReveals = Permission | Permission[] | null;

interface CallCenterItem {
  label: string;
  route: string;
  icon: string;
  permission: CallCenterReveals;
}

interface CallCenterGroup {
  label: string;
  items: CallCenterItem[];
}

/**
 * `subtitle` deliberately keeps the estate's product name: this is another
 * surface of the same product, not a separate one, and the sidebar header
 * should read that way. See docs/ui-design-spec.md §2.
 */
export const CALLCENTER_BRAND: NavBrand = {
  title: 'Call Center',
  subtitle: 'WevetelBastion',
  home: '/dashboard',
};

const groups: CallCenterGroup[] = [
  {
    label: 'Reports',
    items: [
      {
        label: 'Dashboard',
        route: '/dashboard',
        icon: 'grid',
        permission: 'call-center.dashboard.index',
      },
      {
        label: 'Answered Calls',
        route: '/answered-calls',
        icon: 'phone-call',
        permission: 'call-center.answered-calls.index',
      },
      {
        label: 'Unanswered Calls',
        route: '/unanswered-calls',
        icon: 'phone-missed',
        permission: 'call-center.unanswered-calls.index',
      },
      {
        label: 'Call Search',
        route: '/call-search',
        icon: 'search',
        permission: 'call-center.call-search.index',
      },
      {
        label: 'Distribution',
        route: '/distribution',
        icon: 'panel',
        permission: 'call-center.distribution.index',
      },
      {
        label: 'Agent Performance',
        route: '/agent-performance',
        icon: 'user',
        permission: 'call-center.agent-performance.index',
      },
      {
        label: 'Realtime Monitor',
        route: '/realtime-monitor',
        icon: 'grid',
        permission: 'call-center.realtime-monitor.index',
      },
    ],
  },
  {
    label: 'Administration',
    items: [
      {
        label: 'Users',
        route: '/users',
        icon: 'users',
        permission: 'call-center.users.index',
      },
      {
        label: 'SIP Extensions',
        route: '/sip-extensions',
        icon: 'phone-call',
        permission: 'call-center.sip-extensions.index',
      },
      {
        label: 'User Filters',
        route: '/user-filters',
        icon: 'sliders',
        permission: 'call-center.user-filters.index',
      },
      {
        label: 'Settings',
        route: '/settings',
        icon: 'sliders',
        permission: 'call-center.settings.index',
      },
      {
        label: 'Queue Groups',
        route: '/queue-groups',
        icon: 'grid',
        permission: 'call-center.queue-groups.index',
      },
      {
        label: 'Scheduled Reports',
        route: '/scheduled-reports',
        icon: 'clock',
        permission: 'call-center.scheduled-reports.index',
      },
    ],
  },
];

export const CALLCENTER_NAV: NavGroup[] = groups;
