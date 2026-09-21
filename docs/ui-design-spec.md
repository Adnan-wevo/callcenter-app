# CallCenter UI — design spec

> **§11 supersedes parts of §1–§9.** This document was first written against
> the `wevetel-bastion` console. The target has since changed to **heal-crm**
> — the app these screens are being extracted *from*, whose users already
> know them. Where the two disagree, heal-crm wins. §11 lists the
> differences; the structural guidance elsewhere (folder layout, guards, API
> client) still stands, because that came from bastion and is unaffected.

The CallCenter frontend must read as the same application as
`wevetel-bastion`'s console: same chrome, same logo placement, same wording
register, same way things open, same search/filter grammar.

This document records those conventions as they actually exist in bastion, so
they can be reproduced rather than re-invented. Every pattern below was read
from the live source; the file it comes from is named so it can be checked.

See also [extraction-plan.md](extraction-plan.md) §7 for the code-level
architecture (auth, API client, guards). This document is the *visual and
interaction* half.

---

## 0. First decision — separate app, or another surface?

Bastion already has this exact situation solved. `/helpdesk`, `/support` and
`/rnd-sdlc` are **not separate apps**: they keep the same shell, header,
theme toggle, sign-out and permission machinery, and only **swap the
sidebar** (`app.ts` → `subSystem()` → `navGroups()` / `navBrand()`). Moving
between them is a navigation, not a sign-in.

So there are two ways to build CallCenter:

| Option | What it means |
|---|---|
| **A — a surface inside bastion** | Add `CALLCENTER_NAV` + `CALLCENTER_BRAND`, mount routes under `/call-center`, point its feature services at this Go service. Identical look for free; one login; one deploy. |
| **B — its own Angular app** | Copy `core/`, `layout/` and `shared/` out of bastion into `callcenter-app/web/`. Full independence; the design must then be kept in step by hand, and it drifts. |

**Recommendation: A**, unless there is a reason CallCenter must deploy or
authenticate independently. Everything below applies either way — under B it
is a copy, under A it is reuse.

---

## 1. Shell

From `app.html` and `wevetel-sidebar.ts`. Structure, outside in:

```
hlmSidebarWrapper                    h-svh, overflow-hidden
  --header-height: --spacing(14)     (3.5rem)
  --content-height: calc(100svh - var(--header-height))

  ├── <header>                       sticky top-0 z-50, h-(--header-height), border-b, pr-4
  │     ├── sidebar toggle           in a box exactly --sidebar-width-icon wide
  │     ├── vertical separator       h-4
  │     ├── breadcrumb
  │     └── theme toggle             ml-auto
  │
  └── flex min-h-0 flex-1
        ├── hlm-sidebar              collapsible="icon", top-(--header-height)
        └── <main hlmSidebarInset>   h-(--content-height) min-h-0 overflow-hidden
              └── scroll box         h-full min-h-0 overflow-y-auto p-4
                    └── measure      mx-auto min-h-full w-full lg:w-[80%] flex-col
                          ├── router-outlet wrapper (flex-1)
                          └── footer
```

Three rules worth stating because they are easy to get wrong:

- **Only the content area scrolls.** The header stays put, and so does a
  table's own header and pager. That is what `h-(--content-height)` +
  `min-h-0` buys — without `min-h-0` a flex child refuses to shrink, the main
  grows past the viewport, and the whole page scrolls with the header sliding
  away.
- **The measure is 80% from `lg` up**, centred, with the scrollbar left on
  the viewport edge (centre the *content*, do not narrow the *scroller*).
  Below `lg` it is full width — a 60% measure on a phone is a column of
  wrapped text between two empty margins.
- **The footer is pinned to the bottom of the screen on short pages** via
  `min-h-full` + `flex-1` on the outlet wrapper, and simply falls at the end
  of the content on tall ones.

---

## 2. Logo and brand

The logo is **not** a standalone mark — it lives inside the workspace
switcher, in the sidebar header (`wevetel-sidebar.ts` →
`<wevetel-workspace-switcher>`, `workspace-switcher.ts`).

```html
<!-- With a logo file: the mark sits on the plain background, because it
     carries its own colour. -->
<div class="bg-background flex aspect-square size-8 shrink-0 items-center
            justify-center overflow-hidden rounded-lg">
  <img [src]="logo" alt="" class="size-7 object-contain" />
</div>

<!-- Without one: an icon on the brand colour. -->
<div class="bg-brand text-brand-foreground flex aspect-square size-8
            shrink-0 items-center justify-center rounded-lg">
  <ng-icon ... />
</div>

<!-- Beside it, stacked: -->
<span class="truncate font-semibold">{{ brand().title }}</span>
<span class="text-muted-foreground truncate text-xs">{{ brand().subtitle }}</span>
```

Brand shape (`central-nav.ts`):

```ts
export const CENTRAL_BRAND = {
  title: 'Central Console',      // what this surface is
  subtitle: 'WevetelBastion',    // what product it belongs to
  home: '/dashboard',            // where the tile navigates
  logo: 'wevetel.png',           // from web/public; optional
};
```

**For CallCenter:**

```ts
export const CALLCENTER_BRAND = {
  title: 'Call Center',
  subtitle: 'WevetelBastion',   // keep — it is the same product
  home: '/call-center/dashboard',
  logo: 'wevetel.png',
};
```

Note the rule bastion follows deliberately: a **tenant** portal has *no*
logo, because showing the estate's mark on a customer's portal would tell
them they are somewhere they are not. CallCenter is an internal surface, so
it keeps the mark.

---

## 3. Wording

The register is plain, declarative, lower-case-after-first-word. Observed
conventions:

| Slot | Rule | Examples from bastion |
|---|---|---|
| Page title | The thing, in title case, no verb | `Remote Sites`, `Departments` |
| Page description | One line, says what the screen *is*, ends with a full stop | "Portal URLs a disposable Chrome pod opens through a workspace tunnel." |
| Primary button | Verb + noun | `Add site`, `Save`, `Retry` |
| Secondary button | Noun alone | `Filters`, `Actions`, `Cancel` |
| Search placeholder | `Search <what>…` with a real ellipsis character | `Search label or host…` |
| Empty select option | `Any <thing>` | `Any workspace` |
| Filter reset | `Clear all` | — |
| Dialog titles | Never the HTTP status | `Done` / `Something went wrong` / `Just so you know` |
| Error screens | Say what happened *and* what is not assumed | "We reached the server but could not resolve what you are allowed to do. Nothing is assumed — until this succeeds, no action is available." |

**Proposed CallCenter titles and descriptions:**

| Screen | Title | Description |
|---|---|---|
| Dashboard | `Dashboard` | Answered, unanswered and per-queue distribution over the selected window. |
| Answered | `Answered Calls` | Calls a queue connected to an agent, with wait, hold and talk time. |
| Unanswered | `Unanswered Calls` | Calls that left a queue without being answered, and why. |
| Call search | `Call Search` | Find one call by number, id or duration, and read its full timeline. |
| Agent performance | `Agent Performance` | Session, pause, talk and occupancy figures per agent. |
| Distribution | `Distribution` | Received, answered, abandoned and SLA per queue. |

---

## 4. Page skeleton

Every collection screen opens the same way (`page-header.ts`, and
`sites-list.html` as the reference implementation).

```html
<app-page-header
  title="Unanswered Calls"
  description="Calls that left a queue without being answered, and why."
  [hasActions]="canExport()"
>
  <button hlmBtn variant="outline" type="button" (click)="toggleActionsMenu()">
    <ng-icon name="lucideEllipsisVertical" class="mr-2 text-base" />
    Actions
  </button>
</app-page-header>
```

`hasActions` is a separate input rather than content-detection: the row must
not render — not even as empty padding — when every action on the page is
gated away from the caller.

Actions sit **below** the title, on their own row, not beside it. Sharing a
row makes buttons compete with the heading for width and run off a narrow
viewport.

The actions themselves are individually gated:

```html
@if (actionsMenuOpen()) {
  <div class="mt-3 flex shrink-0 flex-wrap items-center gap-2 rounded-md border p-3">
    <button *hasPermission="'call-center.unanswered-calls.export'"
            hlmBtn type="button" (click)="exportCsv()">
      <ng-icon name="lucideDownload" class="mr-2 text-base" />
      Export CSV
    </button>
  </div>
}
```

---

## 5. Search and filters

This is the pattern the user specifically asked to match. Three parts, in
order, from `sites-list.html`.

### 5.1 Toolbar — search box + Filters toggle

```html
<div class="mt-6 flex shrink-0 items-center gap-2">
  <div class="relative flex-1">
    <ng-icon
      name="lucideSearch"
      class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-base"
    />
    <input
      hlmInput
      type="search"
      class="w-full pl-9"
      placeholder="Search caller, queue or agent…"
      aria-label="Search unanswered calls"
      [value]="search()"
      (input)="onSearchInput($event)"
    />
  </div>
  <button hlmBtn variant="outline" type="button" (click)="toggleFilterPanel()">
    <ng-icon name="lucideSlidersHorizontal" class="mr-2 text-base" />
    Filters
    @if (activeFilterTags().length > 0) {
      <span hlmBadge variant="secondary" class="ml-2 text-[0.65rem]">
        {{ activeFilterTags().length }}
      </span>
    }
  </button>
</div>
```

The magnifier is **inside** the input (absolute, `left-3`, `pointer-events-none`,
input gets `pl-9`), not a button beside it. The Filters button carries a
count badge when any filter is active, so a filtered view is never silently
filtered.

Search input is debounced by the component (bastion uses `debounceTime` +
`distinctUntilChanged` on a `Subject`).

### 5.2 Inline filter panel

Opens *in place*, pushing content down — not a popover, not a drawer.
Every control applies immediately; there is no Apply button.

```html
@if (filterPanelOpen()) {
  <div class="mt-3 grid shrink-0 gap-4 rounded-md border p-4">
    <div class="flex items-center justify-between">
      <span class="text-sm font-semibold">Filters</span>
      <button hlmBtn variant="outline" size="sm" type="button" (click)="clearAllFilters()">
        Clear all
      </button>
    </div>

    <div class="grid gap-4 md:grid-cols-2">
      <!-- a closed set = a row of toggle buttons, selected one is variant="default" -->
      <div class="grid gap-2">
        <span class="text-sm font-medium">Grouping</span>
        <div class="flex flex-wrap gap-2">
          @for (g of groupings; track g.value) {
            <button hlmBtn size="sm" type="button"
                    [variant]="filters().grouping === g.value ? 'default' : 'outline'"
                    (click)="setGrouping(g.value)">
              {{ g.label }}
            </button>
          }
        </div>
      </div>

      <!-- an open/long set = a select, with an "Any …" empty option -->
      <div class="grid gap-1.5">
        <span class="text-sm font-medium">Queue group</span>
        <select class="border-input bg-background h-9 rounded-md border px-2 text-sm"
                aria-label="Filter by queue group"
                [value]="filters().queue_group_id"
                (change)="onQueueGroupChange($event)">
          <option value="">Any queue group</option>
          @for (qg of queueGroups(); track qg.id) {
            <option [value]="qg.id">{{ qg.name }}</option>
          }
        </select>
      </div>
    </div>
  </div>
}
```

Rule for picking the control: **closed, short set → toggle buttons; open or
long set → select.** Never a free-text box for a value the server matches
exactly — a fragment typed against an exact match returns nothing for a value
that exists, which reads as "no such rows".

### 5.3 Active filter chips

Below the panel, always visible (even when the panel is closed), each
removable:

```html
@if (activeFilterTags().length > 0) {
  <div class="mt-3 flex shrink-0 flex-wrap items-center gap-2">
    @for (tag of activeFilterTags(); track tag.value) {
      <span hlmBadge variant="secondary" class="gap-1.5 pr-1.5 text-xs">
        {{ tag.label }}
        <button type="button" class="hover:bg-muted-foreground/20 rounded-full p-0.5"
                [attr.aria-label]="'Remove filter ' + tag.label"
                (click)="removeFilterTag(tag)">
          <ng-icon name="lucideX" class="text-[0.7rem]" />
        </button>
      </span>
    }
  </div>
}
```

### 5.4 CallCenter's filter bar is different — and that matters

Bastion filters are mostly closed sets of chips. CallCenter's shared filter
(`HasReportFilters`) is a different shape, and it is on **every** report
screen:

| Filter | Control |
|---|---|
| `date_from` / `date_to` | Two date inputs, side by side — the `date-range` pattern the data-table already uses |
| `seconds_start` / `seconds_end` | Time-of-day sub-range; two `<input type="time">` |
| `queues[]` | Multi-select (long, open set) |
| `agents[]` | Multi-select (long, open set) |
| `queue_group_id` | Select with `Any queue group` |
| `grouping` | Toggle buttons — closed set of 11 |

Because it appears on six screens, build it **once** as a shared component
(`shared/components/report-filters/`) — it is the Angular equivalent of the
`HasReportFilters` trait. It emits one `ReportFilters` object; each screen
passes it to its own service call.

Two behaviours it must carry, from the Laravel side (see extraction-plan
§4.3):

- **The 90-day clamp is silent.** If the user picks a wider range the server
  narrows it. The UI should say so — a chip or a hint reading
  `Range limited to 90 days` — rather than showing figures for a window the
  user did not ask for.
- **Dates are local (`Asia/Kuala_Lumpur`), not UTC.** Send naive
  `YYYY-MM-DD HH:mm:ss`. Do not let the browser serialise a `Date` to an ISO
  string with a `Z`.

---

## 6. How things open

| Kind | Component | Behaviour |
|---|---|---|
| Create / edit form | `app-crud-drawer` | Slides from the **right** by default. The **page owns `open`** — the drawer never closes itself, so a failed save keeps it open with the field errors showing and the user's input intact. Footer: `Cancel` (outline) + `Save` (default, spinner while saving). Body scrolls on its own so a long form never pushes Save off the bottom. |
| Read-only detail | `app-show-drawer` | Same frame, no save. |
| Destructive confirm | `app-confirm-dialog` | Modal. |
| Feedback | The single `hlm-alert-dialog` in `app.html` | One slot for everything: success, info, and every failed request via the error interceptor. |

**Why a drawer and not a route:** a create form used to be its own route
(`/sites/new`), which meant leaving the list to add a row and coming back to
a list that had to reload. The drawer keeps the list on screen behind the
form.

**For CallCenter** the only write is the callback action, so drawers are
mostly for reading:

- **Call detail** (from Call Search, and from any row carrying a `uniqueid`)
  → `app-show-drawer`, rendering the timeline array the `call-detail` action
  returns. This mirrors Livewire's existing modal exactly.
- **Callback** → not a drawer at all. It is a row action with a confirm, then
  a POST. See §8.

---

## 7. Tables

`app-data-table` (`data-table.ts` / `.html`): the `thead` is Angular's, the
`tbody` is DataTables'. Everything with a binding or an event lives in the
head, so nothing Angular owns gets rewritten by a redraw.

- Page lengths: `10, 50, 100, 500, 1000`.
- Optional per-column filter row (`select`, `text`, or `date-range` — the
  last sends `from..to` as one term).
- `responsivePriority` decides which columns fold first on a narrow viewport;
  `DT_FOLD_FIRST` (9000) marks "cheapest column to give up", typically a
  timestamp.
- A failed refresh renders an error strip **above** the table and leaves the
  existing rows on screen — blanking them would suggest the collection is
  now empty.

**CallCenter caveat:** only the *detail* tabs are tabular and paginated.
Summary and grouped responses are keyed maps, not lists — render those as
stat tiles and grouped tables, not through the paginated data-table.

---

## 8. Row action: Callback

The one interactive element unique to CallCenter. Today in Livewire it does
two things on one click: records an attempt, then dispatches a browser event
that makes the softphone dial.

UI shape, matching bastion's conventions:

```html
<button *hasPermission="'call-center.unanswered-calls.callback'"
        hlmBtn variant="outline" size="sm" type="button"
        (click)="confirmCallback(row)">
  <ng-icon name="lucidePhoneCall" class="mr-2 text-base" />
  Call back
</button>
```

Then a confirm dialog (`Call back +60123456789?`), then the POST. On success
the single alert dialog says `Done`.

**Open:** the dial half. A REST call cannot dispatch the Livewire browser
event that drives the softphone widget. This is decision **D7** in the
extraction plan and it is a UI decision as much as a backend one — whether
the Angular app embeds the softphone, deep-links to it, or the button only
records the attempt and the agent dials manually.

---

## 8b. Folder convention

Taken from bastion and followed exactly, so someone who knows one codebase
can navigate the other.

```
src/app/
  core/                         cross-cutting, never feature-specific
    api/        api.service.ts, api.types.ts   (the single HTTP client)
    auth/       auth.service.ts, auth.guard.ts (credential + session)
    authz/      permission.store.ts, permission.guard.ts,
                has-permission.directive.ts, permissions.ts
    interceptors/  auth.interceptor.ts, error.interceptor.ts

  layout/                       the shell
    nav.ts                      TYPES only + NAV_AUTHORITY token
    callcenter-nav.ts           the surface's brand + nav CONFIG
    callcenter-sidebar.ts       sidebar, collapsible sections, slots
    callcenter-breadcrumb.ts    header breadcrumb
    callcenter-footer.ts

  shared/                       reused by more than one feature
    components/<name>/<name>.ts (+ .html)   one folder PER component
    services/<name>.service.ts

  features/<feature>/
    <feature>.routes.ts         exports UPPER_SNAKE_ROUTES
    <feature>.service.ts        the feature's own API calls
    <feature>-list/
      <feature>-list.ts
      <feature>-list.html       separate file once the template is large

  environments/environment.ts
```

Four rules worth stating, because each is something bastion does
deliberately rather than by accident:

- **`nav.ts` holds types, `<surface>-nav.ts` holds config.** The split is
  what lets one shell render a surface whose permission catalogue it does not
  know, while each surface still gets its own literals compile-checked
  against the generated `Permission` union.
- **Every shared component gets its own folder**, even a 30-line one. That is
  what makes adding a `.html`, a `.spec.ts` or a sibling later a non-event
  instead of a move.
- **A feature's service lives in the feature**, not in `core/api/`. Only a
  service used by more than one feature is promoted to `core/`.
- **Route files export an `UPPER_SNAKE_ROUTES` const** and are lazy-loaded
  from the parent with `loadChildren`.

## 9. Theme

From `styles.css`. Spartan/shadcn token set in **oklch**, light and dark,
plus one brand token registered with Tailwind (`bg-brand`,
`text-brand-foreground`):

```css
:root       { --brand: oklch(0.55 0.18 240); --brand-foreground: oklch(0.985 0 0); }
:root.dark  { --brand: oklch(0.65 0.18 240); --brand-foreground: oklch(0.145 0 0); }
```

`--radius: 0.625rem`. System font stack, no webfont. Dark mode is class-based
(`:root.dark`) driven by `ThemeService`, with the toggle in the header.

Keep the same token names and values — that is what makes two surfaces look
like one product. If CallCenter ever needs its own accent, change `--brand`
and nothing else.

---

## 10. Build order

1. Shared `report-filters` component (§5.4) — six screens depend on it.
2. Dashboard — proves the shell, brand, filter bar and stat tiles end to end.
3. Unanswered Calls — adds the detail table, export and the callback action.
4. Answered Calls, Distribution — same shape as Unanswered.
5. Call Search — adds the detail drawer.
6. Agent Performance — three panels, no table.

Each screen ships with its route guard (`requirePermission`) and its nav
entry gated the same way, or it is not done.

---

## 11. heal-crm is the visual target

The screens being replaced already exist in heal-crm as Livewire/Blade views
built on **Flux UI**, and the people who will use the Angular versions use
those today. Matching them is not a preference — it is the difference
between a port and a retraining exercise.

Source of truth: `Modules/CallCenter/resources/views/livewire/*/index.blade.php`
and `resources/views/layouts/app.blade.php` in heal-crm.

### 11.1 What changed from the bastion-derived spec

| | Bastion (§1–§9) | **heal-crm (authoritative)** |
|---|---|---|
| Brand | `oklch(0.55 0.18 240)` | **`#1b96c6`**, hardcoded across its Blade views, same shade in dark mode |
| Neutrals | oklch grey scale | **Tailwind `zinc-*`** — `border-zinc-200 dark:border-zinc-700`, `bg-white dark:bg-zinc-800`, content area `bg-zinc-50 dark:bg-zinc-900` |
| Radius | `0.625rem` (`rounded-lg`) | **`0.75rem`** (`rounded-xl`) on cards and filter panels |
| Page header | Title + description | **Coloured icon + bold title**, description under, closed by a `border-b pb-4` rule |
| Filters | A `Filters` toggle button opening a panel | **Always-visible filter card**, applying immediately, labelled `FILTERS` in uppercase `tracking-widest` beside a funnel icon |
| Filter controls | Chips for closed sets | **Labelled fields in a grid** (`sm:grid-cols-2 xl:grid-cols-4`): Start Date, End Date, Queue, Queue Group |
| Primary button | Neutral `--primary` | **The brand colour**, with `#1580ab` on hover and a small shadow |
| Content padding | `p-4` | **`p-6`** |
| Header bar | Sidebar trigger + breadcrumb + theme toggle | **Breadcrumb from URL segments + a live clock** (`en-MY`, 12-hour, seconds), `h-14` |
| Footer | Docs links | **Copyright + "by Wevetel Sdn. Bhd."**, brand-coloured link |

The page icon is not decoration: heal-crm colours it per screen (Unanswered
Calls uses a red `phone-x-mark`), so the report is recognisable before the
title is read. `app-page-header` therefore takes `icon` and `iconClass`.

### 11.2 The shell is a three-column workspace

heal-crm's layout is not a sidebar and a page. It is:

```
softphone panel (360px, collapsible)  |  content  |  call workspace drawer (640px)
```

Both side panels are `@persist`ed Livewire components driven by **window
events** — `workspace:open-call`, `workspace:minimized`, `workspace:call-ended`,
`workspace:state-sync`. The breadcrumb bar carries a live "active call" pill
showing the number, with a minimise control.

**This is not built here**, and it is the single largest gap between this app
and the thing it replaces. Two consequences:

- The Angular app currently renders sidebar + content only. An agent used to
  having the softphone on the left will notice immediately.
- **It answers D7.** The dial is a browser event to a persisted softphone
  component, not an HTTP call. Whatever this app does about dialling has to
  reach that same component — either by embedding it, or by dispatching the
  same `workspace:open-call` event into a page that still hosts it. A REST
  endpoint alone cannot close this gap, and the confirm dialog currently says
  so out loud rather than pretending otherwise.

### 11.3 Still to match

- Queue and agent **multi-select**: heal-crm uses a custom Alpine dropdown
  with checkboxes, brand-tinted when checked, and a `Clear selection` row.
  This app has no queue/agent filter yet.
- **Flux UI components** (`flux:field`, `flux:input`, `flux:select`,
  `flux:heading`) — this app hand-writes the equivalents in `styles.css`. The
  look is matched; the component API is not, and does not need to be.
- The **softphone and workspace panels** (§11.2).
