import { describe, expect, it } from 'vitest'
import { canManageAssets, canManageUsers, hasAnyRole, hasRole } from './permissions'

describe('role permissions', () => {
  it('enforces administrator-only user management', () => {
    expect(canManageUsers({ role: 'admin' })).toBe(true)
    expect(canManageUsers({ role: 'asset_manager' })).toBe(false)
  })

  it('allows asset managers but not applicants to mutate assets', () => {
    expect(canManageAssets({ role: 'asset_manager' })).toBe(true)
    expect(canManageAssets({ role: 'applicant' })).toBe(false)
  })

  it('handles empty and explicit role sets', () => {
    expect(hasRole({ role: 'approver' }, 'approver')).toBe(true)
    expect(hasAnyRole(null, [])).toBe(true)
    expect(hasAnyRole({ role: 'applicant' }, ['admin'])).toBe(false)
  })
})
