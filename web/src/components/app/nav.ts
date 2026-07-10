import type { TabId } from '../../types'
import { canSeeTabByControlPlane } from '../../tabNav'

export const TAB_IDS = [
  'dashboard',
  'battlegroup',
  'players',
  'database',
  'logs',
  'blueprints',
  'bases',
  'guilds',
  'landsraad',
  'storage',
  'livemap',
  'server',
  'director',
  'market',
  'welcome',
  'events',
  'battlepass',
  'permissions',
  'diagnostics',
] as const

export const DEFAULT_TAB: TabId = 'dashboard'

export const currentTabFromPath = (pathname: string): TabId => {
  const seg = pathname.replace(/^\//, '').split('/')[0]
  return (TAB_IDS as readonly string[]).includes(seg) ? (seg as TabId) : DEFAULT_TAB
}

// Lucide icon per top-level tab, shown in the collapsible sidebar rail.
export const TAB_ICONS: Record<TabId, string> = {
  dashboard: 'layout-grid',
  battlegroup: 'activity',
  logs: 'scroll-text',
  database: 'database',
  server: 'settings-2',
  director: 'clapperboard',
  players: 'users',
  livemap: 'map',
  storage: 'package',
  bases: 'house',
  guilds: 'shield',
  landsraad: 'landmark',
  blueprints: 'scroll',
  market: 'store',
  welcome: 'gift',
  events: 'calendar-clock',
  battlepass: 'medal',
  permissions: 'lock',
  diagnostics: 'stethoscope',
}

// Read-level capability required to see each tab when backend auth is on.
// 'owner' is special: only owners (guild owner, configured owners, local
// account) see the Permissions tab.
export const TAB_CAPABILITIES: Record<TabId, string> = {
  dashboard: 'server:read',
  battlegroup: 'server:read',
  logs: 'logs:read',
  database: 'database:read',
  server: 'config:read',
  director: 'config:read',
  players: 'players:read',
  livemap: 'world:read',
  storage: 'world:read',
  bases: 'world:read',
  guilds: 'players:read',
  landsraad: 'players:read',
  blueprints: 'world:read',
  market: 'market:read',
  welcome: 'welcome:read',
  events: 'events:read',
  battlepass: 'battlepass:track',
  permissions: 'owner',
  // Real capability (not the 'owner' pseudo-cap) so the tab is visible in
  // local/no-auth dev (can() is always true) and, when auth is on, owners
  // bypass the matrix while non-owners need diagnostics:read granted.
  diagnostics: 'diagnostics:read',
}

export const BETA_TABS = new Set<TabId>(['events', 'battlepass'])

export interface CanSeeTabParams {
  key: TabId
  serverCount: number
  authEnabled: boolean
  isOwner: boolean
  can: (capability: string) => boolean
  control: string | undefined
}

// resolveCanSeeTab is the pure decision behind AppCore's canSeeTab — extracted
// so the visibility rules (including the control-plane gate, #262.1) are
// unit-testable without rendering the app shell.
//
// - Dashboard is always visible (home + onboarding surface).
// - Diagnostics stays visible with no servers configured (it's about
//   dune-admin itself), but every other tab is hidden in that state.
// - Control-plane support (e.g. Director requiring AMP) IS a visibility gate:
//   a tab whose capability the session holds still hides on a control plane
//   that can't back it, matching the "not supported" notice the tab itself
//   would otherwise show.
// - Otherwise falls back to the capability matrix; 'owner' is a pseudo-cap
//   gated on authEnabled + (isOwner || auth:manage).
export const resolveCanSeeTab = (params: CanSeeTabParams): boolean => {
  const { key, serverCount, authEnabled, isOwner, can, control } = params
  if (key === 'dashboard') return true
  if (serverCount === 0 && key !== 'diagnostics') return false
  if (!canSeeTabByControlPlane(key, control)) return false
  const cap = TAB_CAPABILITIES[key]
  if (cap === 'owner') return authEnabled && (isOwner || can('auth:manage'))
  return can(cap)
}

export interface NavGroup {
  title: string
  items: { key: TabId, label: string }[]
}
