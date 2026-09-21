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

/**
 * Entries whose screens are not built yet are ABSENT rather than present and
 * dead: a nav item that leads nowhere is worse than a missing one. The rest
 * of the catalogue (Answered Calls, Call Search, Agent Performance,
 * Distribution) joins this list as each screen lands — see
 * docs/ui-design-spec.md §10 for the order.
 */
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
        label: 'Unanswered Calls',
        route: '/unanswered-calls',
        icon: 'phone-missed',
        permission: 'call-center.unanswered-calls.index',
      },
    ],
  },
];

export const CALLCENTER_NAV: NavGroup[] = groups;
