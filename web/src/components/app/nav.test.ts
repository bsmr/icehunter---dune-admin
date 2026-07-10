import { describe, it, expect } from 'vitest'
import { resolveCanSeeTab } from './nav'

describe('resolveCanSeeTab', () => {
  const allow = (): boolean => true
  const deny = (): boolean => false

  it('dashboard is always visible regardless of servers, control, or capability', () => {
    expect(resolveCanSeeTab({
      key: 'dashboard', serverCount: 0, authEnabled: true, isOwner: false, can: deny, control: undefined,
    })).toBe(true)
  })

  it('non-diagnostics tabs are hidden when no servers are configured', () => {
    expect(resolveCanSeeTab({
      key: 'players', serverCount: 0, authEnabled: false, isOwner: false, can: allow, control: 'local',
    })).toBe(false)
  })

  it('diagnostics stays visible with no servers configured (still subject to capability)', () => {
    expect(resolveCanSeeTab({
      key: 'diagnostics', serverCount: 0, authEnabled: false, isOwner: false, can: allow, control: undefined,
    })).toBe(true)
  })

  // #262.1: the Director nav item must be hidden on non-AMP control planes,
  // matching the page-body gate DirectorTab already applies.
  it('hides director on a non-AMP control plane even when capability allows it', () => {
    expect(resolveCanSeeTab({
      key: 'director', serverCount: 1, authEnabled: false, isOwner: false, can: allow, control: 'kubectl',
    })).toBe(false)
  })

  it('shows director on the amp control plane when capability allows it', () => {
    expect(resolveCanSeeTab({
      key: 'director', serverCount: 1, authEnabled: false, isOwner: false, can: allow, control: 'amp',
    })).toBe(true)
  })

  it('hides director while control plane is still resolving (undefined)', () => {
    expect(resolveCanSeeTab({
      key: 'director', serverCount: 1, authEnabled: false, isOwner: false, can: allow, control: undefined,
    })).toBe(false)
  })

  it('tabs unrestricted by control plane are unaffected by it', () => {
    expect(resolveCanSeeTab({
      key: 'players', serverCount: 1, authEnabled: false, isOwner: false, can: allow, control: 'kubectl',
    })).toBe(true)
  })

  it('permissions ("owner" pseudo-capability) requires authEnabled and owner/auth:manage', () => {
    expect(resolveCanSeeTab({
      key: 'permissions', serverCount: 1, authEnabled: true, isOwner: true, can: deny, control: 'local',
    })).toBe(true)
    expect(resolveCanSeeTab({
      key: 'permissions', serverCount: 1, authEnabled: true, isOwner: false, can: deny, control: 'local',
    })).toBe(false)
    expect(resolveCanSeeTab({
      key: 'permissions', serverCount: 1, authEnabled: false, isOwner: true, can: allow, control: 'local',
    })).toBe(false)
  })

  it('regular tabs defer to the can() capability check', () => {
    expect(resolveCanSeeTab({
      key: 'market', serverCount: 1, authEnabled: true, isOwner: false, can: deny, control: 'local',
    })).toBe(false)
    expect(resolveCanSeeTab({
      key: 'market', serverCount: 1, authEnabled: true, isOwner: false, can: allow, control: 'local',
    })).toBe(true)
  })
})
